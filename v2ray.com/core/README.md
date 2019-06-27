### 简要描述

这是 `v2ray.com/core@v4.14` 的一个子集, 只包含了调用 `grpc` 中必要的代码, 
但目前不是所有 `grpc` 接口代码都会放进来, 只会优先放置一些我会用的接口,
如果没有你所需要的可以 `fork` 一份自己修改, 如果能贡献到本仓库就更好了

### 制作原因

因为使用 `v2ray.com/core` 调用接口的话没办法用 `vscode` 调试, 但是只使用 `grpc`
的话是可以调试的, 于是我就把 `v2ray.com/core` 中的 `grpc` 调用的相关代码提取出来放到
这个仓库里了. 
之后替换了 `v2ray.com/core` 再进行调试是没有问题的, 现在算是可以用的 `0.0.x` 版本.

### 使用方法:
```shell
# 替换 v2ray.com/core 为本仓库
go mod edit -replace=v2ray.com/core@v4.14=github.com/shynome/v2ray-go-grpc-sdk@v4.14.1
# 下载
go get -v v2ray.com/core@v4.14
```
调用方法参考 `v2ray.com/core` 的测试文件 [`testing/scenarios/command_test.go`](https://sourcegraph.com/github.com/v2ray/v2ray-core@master/-/blob/testing/scenarios/command_test.go)
