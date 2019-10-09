package reloader

import (
    "os"
    "os/exec"
	"os/signal"
	"syscall"
    "sync"
    "net"
    "log"
    "net/http"
)

/*
基于`net.TCPListener`的无间断重启服务，暴露listener供业务代码扩充

1.`HTTP Server`简单用法：
func main(){
    rl := reloader.NewReloader("127.0.0.1:8080")
    if err:=rl.Bind(); err==nil {
        http.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
            w.Write([]byte("Hello, world!\n"))
        })
        rl.HttpServe(&http.Server{})
    }
}

2.socket server用法
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

重启 `kill -s HUP  ${PID} `
*/

func NewReloader(listen string) (*Reloader) {
    return &Reloader{listen:listen}
}

type Reloader struct {
    *net.TCPListener    //"继承"自net.TCPListener，拥有它全部public方法
    listen string       //监听的"host:port"
    wg sync.WaitGroup   //等待结束用户请求
    http *http.Server   //HTTP-server
}

//"重载"Accept()加wg
func (r *Reloader) Accept() (net.Conn, error) {
    if c,e := r.TCPListener.Accept(); e==nil {
        r.wg.Add(1)
        return ReloaderConn{c,r}, e
    } else {
        return c,e
    }
}

//暴露的net.TCPListener
func (r *Reloader) GetListener() (net.Listener) {
    return interface{}(r).(net.Listener)
}

//自定义端口“绑定”函数，负责读取传递的文件描述符
func (r *Reloader) Bind() (err error) {
    if _,b := os.LookupEnv("RELOADING"); b==true { //child
        if fd := os.NewFile(uintptr(3),""); fd==nil {  //0,1,2标准占用
            log.Panic("child: nil fd!")
        } else if fl,err := net.FileListener(fd); err!=nil {
            log.Panic("child: "+err.Error())
        } else {
            r.TCPListener = fl.(*net.TCPListener)
        }
        syscall.Kill(syscall.Getppid(), syscall.SIGTERM) //通知父进程关闭端口并择机结束
    } else {
        var addr *net.TCPAddr
        if addr,err = net.ResolveTCPAddr("tcp", r.listen); err==nil {
            r.TCPListener, err = net.ListenTCP("tcp", addr)
        }
    }
    r.watch()  //启动信号量监听
    return
}

//使用net/http包Serve()，重启时不需要调用r.Wait()
func (r *Reloader) HttpServe(s *http.Server) {
    r.http = s
    r.http.Serve(r.GetListener())
}

//等待结束用户请求
func (r *Reloader) Wait(){// {{{
    r.wg.Wait()
}// }}}

func (r *Reloader) spawn() {// {{{
    fd,_ := r.File()
	var args []string
	if len(os.Args) > 1 {
		args = os.Args[1:]
	}
    cmd := exec.Command(os.Args[0], args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.ExtraFiles = []*os.File{fd} //fd传递
	cmd.Env = append( os.Environ(), "RELOADING=1", )
    if err := cmd.Start(); err!=nil {
        log.Fatalf("Restart: Failed to launch, error: %v", err)
    }
}// }}}

func (r *Reloader) watch() {// {{{
    go func(){
        sigchan := make(chan os.Signal, 2)
        signal.Notify(sigchan, syscall.SIGHUP, syscall.SIGTERM)
        for {
            switch(<-sigchan) {
            case syscall.SIGTERM:
                signal.Ignore(syscall.SIGHUP, syscall.SIGTERM)
                if r.http != nil {
                    r.http.Shutdown(nil) //会处理http协议的keep-alive
                } else {
                    r.Close()
                }
            case syscall.SIGHUP:
                r.spawn()
            }
        }
    }()
}// }}}

type ReloaderConn struct {// {{{
	net.Conn
	r *Reloader
}

func (c ReloaderConn) Close() error {
	err := c.Conn.Close()
	if err == nil {
		c.r.wg.Done()
	}
	return err
}
// }}}
