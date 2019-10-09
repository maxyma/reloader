# reloader

基于`net.TCPListener`的无间断重启服务，暴露listener供业务代码扩充

1. 基于**HTTP Server**通用方法：
```go
package main

import (
    "net/http"
    "github.com/maxyma/reloader"
)

func main(){
    rl := reloader.NewReloader("127.0.0.1:8080")
    if err:=rl.Bind(); err==nil {
        http.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
            w.Write([]byte("Hello, world!\n"))
        })
        rl.HttpServe(&http.Server{})
    }
}
```

2. **socket server**用法
```go
package main

import (
    "net/http"
    "github.com/maxyma/reloader"
)

func main(){
    rl := reloader.NewReloader("127.0.0.1:9001")
    if err:=rl.Bind(); err==nil {
        http.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
            w.Write([]byte("Hello, world!\n"))
        })
        http.Serve(rl.GetListener(), nil)
        rl.Wait()
    }
}
```

3. 重启
> `$ kill -s HUP  ${PID}`

