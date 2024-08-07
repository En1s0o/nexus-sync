package cmd

import (
	"context"
	"errors"
	"fmt"
	"github.com/panjf2000/ants/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	urlpkg "net/url"
	"nexus-sync/pkg/log"
	"nexus-sync/version"
	"os"
	"strings"
	"sync"
)

var logger log.Logger

func init() {
	logger = log.NewLogger("nexus-sync")
}

type NexusConfig struct {
	URL      string
	User     string
	Password string
	Repo     string
}

// NexusSyncOptions contains configuration flags for the NexusSync.
type NexusSyncOptions struct {
	From NexusConfig
	To   NexusConfig
	pool *ants.Pool
}

// validateNexusSyncOptions validates NexusSync's configuration flags and returns an error if they are invalid.
func validateNexusSyncOptions(opt *NexusSyncOptions) (err error) {
	fromUrl, err := urlpkg.Parse(opt.From.URL)
	if err != nil {
		return err
	}
	toUrl, err := urlpkg.Parse(opt.To.URL)
	if err != nil {
		return err
	}
	if strings.EqualFold(fromUrl.String(), toUrl.String()) &&
		strings.EqualFold(opt.From.Repo, opt.To.Repo) {
		return fmt.Errorf("The same 'from' and 'to' (%s#%s), No-op\n",
			fromUrl.String(), opt.From.Repo)
	}
	return nil
}

// NewNexusSyncCommand creates a *cobra.Command object with default parameters
func NewNexusSyncCommand() *cobra.Command {
	options := &NexusSyncOptions{}

	flags := pflag.CommandLine
	flags.BoolP("help", "h", false, "帮助信息")
	flags.StringVar(&options.From.URL, "from-url", "http://localhost:8081", "源地址")
	flags.StringVar(&options.From.User, "from-user", "admin", "源用户名")
	flags.StringVar(&options.From.Password, "from-pass", "admin123", "源密码")
	flags.StringVar(&options.From.Repo, "from-repo", "maven-releases", "源仓库")
	flags.StringVar(&options.To.URL, "to-url", "http://localhost:8081", "目的地址")
	flags.StringVar(&options.To.User, "to-user", "admin", "目的用户名")
	flags.StringVar(&options.To.Password, "to-pass", "admin123", "目的密码")
	flags.StringVar(&options.To.Repo, "to-repo", "maven-releases", "目的仓库")

	cmd := &cobra.Command{
		Use:                os.Args[0],
		Long:               `很长的描述`,
		DisableFlagParsing: false,
		Run: func(cmd *cobra.Command, args []string) {
			if err := flags.Parse(args); err != nil {
				logger.Error("Failed to parse flag", err)
				_ = cmd.Usage()
				os.Exit(1)
			}

			// check if there are non-flag arguments in the command line
			cmds := flags.Args()
			if len(cmds) > 0 {
				logger.Errorf("Unknown command %s", cmds[0])
				_ = cmd.Usage()
				os.Exit(1)
			}

			// short-circuit on help
			help, err := flags.GetBool("help")
			if err != nil {
				logger.Info(`"help" flag is non-bool, programmer error, please correct`)
				os.Exit(1)
			}
			if help {
				_ = cmd.Help()
				return
			}

			if err := validateNexusSyncOptions(options); err != nil {
				logger.Errorf("Validate options failed: %v", err)
				os.Exit(1)
			}

			// 配置并发池
			pool, err := ants.NewPool(16,
				ants.WithPreAlloc(true),
				ants.WithLogger(log.NewLogger("pool")))
			if err != nil {
				logger.Errorf("NexusSync pool init failed: %v", err)
				os.Exit(1)
			}
			options.pool = pool
			defer pool.Release()

			if err := run(SetupSignalContext(), options); err != nil {
				if errors.Is(err, context.Canceled) {
					logger.Infof("NexusSync canceled")
				} else {
					logger.Errorf("NexusSync failed: %v", err)
				}
				os.Exit(1)
			}
		},
	}

	return cmd
}

func run(ctx context.Context, opt *NexusSyncOptions) error {
	logger.Infof("NexusSync version %s", version.Full())

	newCtx, cancelFunc := context.WithCancel(ctx)
	defer cancelFunc()

	uploadUrl, err := urlpkg.Parse(opt.To.URL)
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	wg.Add(2)

	logger.Info("fetching repo metadata ...")
	// 拉取源 nexus 仓库的所有数据
	var fromItems map[string]*NexusRepositoryItem
	err = opt.pool.Submit(func() {
		defer wg.Done()
		var err error
		logger.Infof("fetching source repo metadata '%s' @ '%s'", opt.From.URL, opt.From.Repo)
		fromItems, err = fetchAll(newCtx, opt.From.URL, opt.From.Repo, opt.From.User, opt.From.Password)
		if err != nil {
			logger.Errorf("Fetch 'from' failed: %v", err)
			cancelFunc()
		} else {
			logger.Infof("fetched source repo metadata '%s' @ '%s'", opt.From.URL, opt.From.Repo)
		}
	})
	if err != nil {
		return err
	}

	// 拉取目的 nexus 仓库的所有数据
	var toItems map[string]*NexusRepositoryItem
	err = opt.pool.Submit(func() {
		defer wg.Done()
		var err error
		logger.Infof("fetching destination repo metadata '%s' @ '%s'", opt.To.URL, opt.To.Repo)
		toItems, err = fetchAll(newCtx, opt.To.URL, opt.To.Repo, opt.To.User, opt.To.Password)
		if err != nil {
			logger.Errorf("Fetch 'to' failed: %v", err)
			cancelFunc()
		} else {
			logger.Infof("fetched destination repo metadata '%s' @ '%s'", opt.To.URL, opt.To.Repo)
		}
	})
	if err != nil {
		return err
	}

	// 等待结束
	wg.Wait()
	select {
	case <-newCtx.Done():
		return newCtx.Err()
	default:
	}
	logger.Info("fetched repo metadata")

	// 计算差异
	diffItems := make(map[string]*NexusRepositoryItem)
	for _, from := range fromItems {
		path := from.Path
		if toItems[path] == nil || toItems[path].SHA1 != fromItems[path].SHA1 {
			diffItems[path] = from
		}
	}
	diffLen := len(diffItems)
	if diffLen == 0 {
		logger.Info("synced no-op")
		return nil
	}

	wg.Add(diffLen)
	for _, item := range diffItems {
		// 调试打印
		logger.Info("diff > ", item.Path)
		// 拼接处完整的路径
		u, err := uploadUrl.Parse("/repository/" + opt.To.Repo + "/" + item.Path)
		if err != nil {
			return err
		}
		item.fullUrl = u.String()
	}

	// 构造传输函数
	transferFunc := func(item *NexusRepositoryItem) (err error) {
		defer wg.Done()
		logger.Info("process ", item.Path)
		for i := 0; i < 3; i++ {
			// 传输：从 from 下载，上传到 to
			err = transfer(ctx, opt, item.DownloadUrl, item.fullUrl)
			if err == nil {
				return
			}
		}
		return
	}

	// 并发传输
	failItems := &sync.Map{}
	for _, item := range diffItems {
		item := item
		err = opt.pool.Submit(func() {
			if transErr := transferFunc(item); transErr != nil {
				failItems.Store(item.Path, transErr)
			}
		})
		if err != nil {
			return err
		}
	}

	// 等待结束
	wg.Wait()
	logger.Info("NexusSync finished")
	failItems.Range(func(key, value interface{}) bool {
		logger.Errorf("NexusSync failed: %s, %v", key, value)
		return true
	})

	return nil
}
