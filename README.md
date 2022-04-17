单体应用的链路运行情况记录

静态页面添加

路由

```
go install github.com/go-bindata/go-bindata/...@latest


go-bindata -o=./views.go -pkg=localtracing ./views/... 


go get github.com/elazarl/go-bindata-assetfs
```

模板

使用go-bindata之后原来的模板函数不能直接使用了

需要包装一下先从godata从获取值然后execute

需要注意的是这里动态创建template的名字需要和模板里的名字对应上否则不能解析


## 示例

examples中的示例

实时日志查看: http://localhost:8080/view?file=log.txt

在对应的log.txt新建日志记录查看效果
