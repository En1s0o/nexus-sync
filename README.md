# 使用说明



## 编译

```shell
go build
```



## 运行

> 示例：把 localhost:8081 的 maven-releases 仓库同步到 example.io 的 maven-releases

```shell
./nexus-sync \
--from-url http://localhost:8081 \
--from-user admin \
--from-pass admin123 \
--from-repo maven-releases \
--to-url http://example.io \
--to-user admin \
--to-pass admin123 \
--to-repo maven-releases
```

