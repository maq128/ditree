# 项目说明

本程序用于以树形列表的方式列示 docker 里面所有的镜像和容器实例。

对于这个应用场景，开源项目 [dockviz](https://github.com/justone/dockviz) 实现的功能更强大，使用更灵活。

本程序仅仅是出于个人偏好，为了得到更喜欢的显示格式。

# 参考资料

[Docker Engine API](https://docs.docker.com/engine/api/)

[github.com/justone/dockviz](https://github.com/justone/dockviz)

# Golang 编译运行
```sh
go get -v github.com/docker/docker/client
go build -o ditree
./ditree
./ditree -a
```

# 构建 docker 镜像
```sh
docker build --tag ditree .

# 如果需要通过代理服务器才能访问 github 的话
docker build --build-arg https_proxy=socks5://x.x.x.x:7070 --tag ditree .
```

# 在 Docker Desktop for Windows 下使用

在 Windows 10 中使用 Decker Desktop 时，Docker Client 缺省是通过 `//./pipe/docker_engine` 连接到 Docker Engine。
本程序所使用的 Docker Engine SDK 可以自动识别出这个 pipe 并使用它，所以本程序在 Windows 命令行下可以直接运行。

但是如果想要在 docker 容器里运行本程序的话就有一点麻烦了，因为这个 pipe 并不能传递到容器里面使用。此时需要在
Docker Desktop 里面开启 `Expose daemon on tcp://localhost:2375 without TLS` 这个选项，然后：
```
docker run -it --rm -e DOCKER_HOST=tcp://host.docker.internal:2375 ditree ditree
```

# 在 Linux 下使用（包括 WSL2）
```sh
docker run --rm -v /var/run/docker.sock:/var/run/docker.sock ditree

# 建立别名，使用更方便
alias ditree='docker run --rm --name=ditree -v /var/run/docker.sock:/var/run/docker.sock ditree ditree'
ditree -a
```

# 关于 image 的 parent

通过 `cli.ImageList()` 得到 `types.ImageSummary`，其中有 `ParentID` 字段，但是这个 parent 并不可靠。

参考 [Docker image parent is empty](https://github.com/moby/moby/issues/22140#issuecomment-211821828)

也就是说，只有在该 image 为本地构建时这个 `ParentID` 才会有效，如果是其它方式获取到的 image，比如 `docker pull`
或者 `docker load`，这个 `ParentID` 会是空的，甚至对应的那个 image 在本地已经存在时也是这样。

经过观察，发现 `cli.ImageInspectWithRaw()` 得到的 `types.ImageInspect` 里面的 `Config.Image` 可以补充作为
parent 来表示 image 之间的依赖关系。
