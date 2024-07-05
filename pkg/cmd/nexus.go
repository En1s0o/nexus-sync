package cmd

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	urlpkg "net/url"
)

type Checksum struct {
	SHA1 string `json:"sha1"`
	MD5  string `json:"md5"`
}

type NexusRepositoryItem struct {
	DownloadUrl string `json:"downloadUrl"`
	Path        string `json:"path"`
	Id          string `json:"id"`
	Repository  string `json:"repository"`
	Format      string `json:"format"`
	Checksum    `json:"checksum"`
	fullUrl     string
}

type NexusRepository struct {
	Items             []*NexusRepositoryItem `json:"items"`
	ContinuationToken string                 `json:"continuationToken"`
}

// fetch 拉取 nexus 仓库数据
func fetch(ctx context.Context, url, username, password string) (*NexusRepository, error) {
	nexusRepository := &NexusRepository{}

	req, err := makeHTTPRequest(ctx, http.MethodGet, url, username, password, nil)
	if err != nil {
		return nil, err
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer func() {
		if err := resp.Body.Close(); err != nil {
			logger.Warnf("[%s] Fetch response close error %v", url, err)
		}
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(body, &nexusRepository)
	if err != nil {
		return nil, err
	}
	return nexusRepository, nil
}

// fetchAll 拉取 nexus 仓库的所有数据
func fetchAll(ctx context.Context, url, repo, username, password string) (map[string]*NexusRepositoryItem, error) {
	items := make(map[string]*NexusRepositoryItem)

	u, err := urlpkg.Parse(url)
	if err != nil {
		return items, err
	}

	u, err = u.Parse("/service/rest/v1/assets")
	if err != nil {
		return items, err
	}

	query := u.Query()
	query.Add("repository", repo)
	u.RawQuery = query.Encode()
	query.Add("continuationToken", "")

	for {
		var repository *NexusRepository
		repository, err = fetch(ctx, u.String(), username, password)
		if err != nil {
			return items, err
		}

		for _, item := range repository.Items {
			items[item.Path] = item
		}

		if repository.ContinuationToken == "" {
			// token 为空表示没有更多数据了
			break
		}
		query.Set("continuationToken", repository.ContinuationToken)
		u.RawQuery = query.Encode()
	}

	return items, nil
}

// transfer 将数据从源仓库传输到目的仓库
func transfer(ctx context.Context, opt *NexusSyncOptions, fromUrl, toUrl string) error {
	newCtx, cancelFunc := context.WithCancel(ctx)
	defer cancelFunc()

	fromReq, err := makeHTTPRequest(newCtx, http.MethodGet, fromUrl, opt.From.User, opt.From.Password, nil)
	if err != nil {
		return err
	}

	reader, writer := io.Pipe()
	defer func() {
		_ = writer.Close()
		_ = reader.Close()
	}()

	toReq, err := makeHTTPRequest(newCtx, http.MethodPut, toUrl, opt.From.User, opt.To.Password, reader)
	if err != nil {
		return err
	}

	go func() {
		defer func() {
			_ = writer.Close()
		}()

		resp, err := httpClient.Do(fromReq)
		if err == nil {
			defer func() {
				if err = resp.Body.Close(); err != nil {
					logger.Warnf("[%s] Download response close error %v", fromUrl, err)
				}
			}()
			_, _ = io.Copy(writer, resp.Body)
		}
	}()

	_, err = httpClient.Do(toReq)
	return err
}
