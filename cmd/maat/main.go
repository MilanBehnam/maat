package main

import (
	"flag"
	"fmt"
	"github.com/lireza/lib/configuring"
	"log"
	"net/http"
	"net/http/httputil"
	netUrl "net/url"
	"sync"
	"sync/atomic"
)

var configFile = flag.String("config", "./configuration/maat.json", "Configuration file")

//Backend encapsulate all necessary data for each peer.
type Backend struct {
	alive bool
	url   *netUrl.URL
	proxy *httputil.ReverseProxy
	mux   sync.RWMutex
}

//ServerPool encapsulate all backend instances and next peer who ready for next request.
type ServerPool struct {
	Backends []*Backend
	Current  uint64
}

var serverPool ServerPool

func main() {
	flag.Parse()
	json := getConfiguration()
	createServerPool(getPeersList(json))
	startServer(getPortFromConfig(json))
}

func getPeersList(json *configuring.Config) *[]string {
	servers, err := json.Get("peers").SliceOfString()
	if err != nil {
		panic(err)
	}
	return &servers
}

func getPortFromConfig(json *configuring.Config) *int {
	port, err := json.Get("port").Int()
	if err != nil {
		panic(err)
	}
	return &port
}

func getConfiguration() *configuring.Config {
	config := configuring.New()
	json, err := config.LoadJSON(*configFile)
	if err != nil {
		panic(err)
	}
	return json
}

func createServerPool(servers *[]string) {
	for _, server := range *servers {
		parse, _ := netUrl.Parse(server)
		proxy := httputil.NewSingleHostReverseProxy(parse)
		serverPool.addBackend(&Backend{alive: true, url: parse, proxy: proxy})
		proxy.ErrorHandler = errHandler
	}
}

func errHandler(w http.ResponseWriter, r *http.Request, err error) {
	//TODO add some retries here.
	fmt.Println(err)
	serverPool.AliveByUrl(r.URL, false)
	loadBalancer(w, r)
}

func startServer(port *int) {
	server := http.Server{
		Addr:    fmt.Sprintf(":%d", *port),
		Handler: http.HandlerFunc(loadBalancer),
	}

	log.Printf("Load Balancer started at :%d\n", *port)
	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}

func loadBalancer(w http.ResponseWriter, r *http.Request) {
	peer := serverPool.getNextAlivePeer()

	if peer != nil {
		peer.proxy.ServeHTTP(w, r)
		return
	}
	http.Error(w, "Service not available", http.StatusServiceUnavailable)

}

func (sp *ServerPool) nextIndex() int {
	return int(atomic.AddUint64(&sp.Current, uint64(1)) % uint64(len(sp.Backends)))
}

func (sp *ServerPool) getNextAlivePeer() *Backend {
	index := sp.nextIndex()
	loop := index + len(sp.Backends)

	for i := index; i < loop; i++ {
		idx := i % len(sp.Backends)
		if sp.Backends[idx].IsAlive() {
			if i != index {
				sp.setCurrent(idx)
			}
			return sp.Backends[idx]
		}
	}
	return nil
}

func (sp *ServerPool) setCurrent(current int) {
	atomic.StoreUint64(&sp.Current, uint64(current))
}

func (sp *ServerPool) addBackend(bk *Backend) {
	sp.Backends = append(sp.Backends, bk)
}

func (b *Backend) IsAlive() (alive bool) {
	b.mux.RLock()
	alive = b.alive
	b.mux.RUnlock()
	return
}

func (b *Backend) setAlive(alive bool) {
	b.mux.Lock()
	b.alive = alive
	b.mux.Unlock()
}

func (sp *ServerPool) AliveByUrl(url *netUrl.URL, alive bool) {
	for _, backend := range sp.Backends {
		if backend.url == url {
			backend.setAlive(alive)
			return
		}

	}
}
