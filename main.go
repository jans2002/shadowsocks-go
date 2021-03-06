package main

import (
	"flag"
	"log"
	"os"
	"sync"
	"time"

	"net/http"
	_ "net/http/pprof"

	ss "github.com/ccsexyz/shadowsocks-go/shadowsocks"
	"github.com/fsnotify/fsnotify"
)

func main() {
	log.SetFlags(log.Lshortfile | log.Ldate | log.Ltime | log.Lmicroseconds)

	var c ss.Config
	var target string
	var configfile string
	var pprofaddr string

	flag.StringVar(&c.Type, "type", "", "server type(eg: server, local)")
	flag.StringVar(&c.Remoteaddr, "s", "", "remote server address")
	flag.StringVar(&c.Localaddr, "l", "", "local listen address")
	flag.StringVar(&target, "t", "", "target address(for tcptun and udptun)")
	flag.StringVar(&configfile, "c", "", "the configuration file path")
	flag.StringVar(&c.Method, "m", "aes-256-cfb", "crypt method")
	flag.StringVar(&c.Password, "p", "you need a password", "password")
	flag.BoolVar(&c.Nonop, "nonop", false, "enable this to be compatiable with official ss servers(client only)")
	flag.BoolVar(&c.UDPRelay, "udprelay", false, "relay udp packets")
	flag.BoolVar(&c.Mux, "mux", false, "use mux to reduce the number of connections")
	flag.StringVar(&c.Nickname, "name", "", "nickname for logging")
	flag.StringVar(&c.LogFile, "log", "", "set the path of logfile")
	flag.BoolVar(&c.Verbose, "verbose", false, "show verbose log")
	flag.BoolVar(&c.Debug, "debug", false, "show debug log")
	flag.StringVar(&pprofaddr, "pprof", "", "the pprof listen address")
	flag.IntVar(&c.Timeout, "timeout", 0, "set the timeout of tcp connection")
	flag.Parse()

	if len(os.Args) == 1 {
		flag.Usage()
		return
	}

	if len(pprofaddr) != 0 {
		go func() {
			log.Println(http.ListenAndServe(pprofaddr, nil))
		}()
	}

	if len(configfile) == 0 {
		if len(target) != 0 {
			c.Backend = &ss.Config{Method: c.Method, Password: c.Password, Remoteaddr: c.Remoteaddr}
			c.Remoteaddr = target
		}
		ss.CheckConfig(&c)
		runServer(&c)
		return
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()

	configs, err := ss.ReadConfig(configfile)
	if err != nil {
		log.Fatal(err)
	}
	for {
		die := make(chan bool)
		var wg sync.WaitGroup
		for _, c := range configs {
			wg.Add(1)
			go func(c *ss.Config) {
				defer wg.Done()
				c.Die = die
				runServer(c)
			}(c)
		}
		go func() {
			err = watcher.Add(os.Args[1])
			if err != nil {
				return
			}
			defer watcher.Remove(os.Args[1])
			for {
				select {
				case event := <-watcher.Events:
					if event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Rename == fsnotify.Rename {
						newConfigs, err := ss.ReadConfig(os.Args[1])
						if err != nil {
							continue
						}
						configs = newConfigs
						close(die)
						return
					} else if event.Op&fsnotify.Remove == fsnotify.Remove {
						return
					}
				case <-watcher.Errors:
					// close(die)
					// return
				case <-die:
					return
				}
			}
		}()
		wg.Wait()
		select {
		case <-die:
		default:
			return
		}
		time.Sleep(time.Second)
		for _, c := range configs {
			c.Close()
		}
	}
}

func runServer(c *ss.Config) {
	switch c.Type {
	default:
		log.Println("unsupported server type:", c.Type)
	case "local":
		c.Log("run client at", c.Localaddr, "with method", c.Method)
		if c.UDPRelay {
			c.Log("run udp local server at", c.Localaddr, "with method", c.Method)
			go RunUDPLocalServer(c)
		}
		RunTCPLocalServer(c)
	case "redir":
		c.Log("run redir at", c.Localaddr, "with method", c.Method)
		RunTCPRedirServer(c)
	case "server":
		c.Log("run server at", c.Localaddr, "with method", c.Method)
		if c.UDPRelay {
			c.Log("run udp remote server at", c.Localaddr, "with method", c.Method)
			go RunUDPRemoteServer(c)
		}
		RunTCPRemoteServer(c)
	case "multiserver":
		c.Log("run multi server at", c.Localaddr)
		if c.UDPRelay {
			c.Log("run multi udp remote server at", c.Localaddr)
			go RunMultiUDPRemoteServer(c)
		}
		RunMultiTCPRemoteServer(c)
	case "ssproxy":
		if c.UDPRelay {
			c.Log("run udp remote proxy server at", c.Localaddr)
			go RunUDPRemoteServer(c)
		}
		c.Log("run ss proxy at", c.Localaddr, "with method", c.Method)
		RunSSProxyServer(c)
	case "socksproxy":
		if c.UDPRelay {
			c.Log("run udp local proxy server at", c.Localaddr, "with method", c.Method)
			go RunUDPLocalServer(c)
		}
		c.Log("run socks proxy at", c.Localaddr, "with method", c.Method)
		RunSocksProxyServer(c)
	case "tcptun":
		if len(c.Localaddr) == 0 || c.Backend == nil || len(c.Backend.Remoteaddr) == 0 {
			break
		}
		c.Log("run tcp tunnel at", c.Localaddr, "to", c.Remoteaddr)
		RunTCPTunServer(c)
	case "udptun":
		if len(c.Localaddr) == 0 || c.Backend == nil || len(c.Backend.Remoteaddr) == 0 {
			break
		}
		c.Log("run udp tunnel at", c.Localaddr, "to", c.Remoteaddr)
		RunUDPTunServer(c)
	}
}
