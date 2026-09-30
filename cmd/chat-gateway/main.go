// chat-gateway: XEP-0357 push app server (Web Push, Acrobits PNM) and the Acrobits web services.
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nethesis/nethvoice-chat/internal/api"
	"github.com/nethesis/nethvoice-chat/internal/push"
	"github.com/nethesis/nethvoice-chat/internal/store"
	"github.com/nethesis/nethvoice-chat/internal/svcauth"
	"github.com/nethesis/nethvoice-chat/internal/xmppc"
)

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func main() {
	log.SetFlags(0)
	host := os.Getenv("CHAT_HOST")
	secret := os.Getenv("CHAT_COMPONENT_SECRET")
	if host == "" || secret == "" {
		log.Fatal("CHAT_HOST and CHAT_COMPONENT_SECRET are required")
	}
	listen := env("CHAT_LISTEN", "127.0.0.1:8080")
	c2s := env("CHAT_C2S_ADDR", "127.0.0.1:5222")
	componentAddr := env("CHAT_COMPONENT_ADDR", "127.0.0.1:5275")
	mwURL := env("CHAT_MIDDLEWARE_URL", "http://127.0.0.1:20038")
	dataDir := env("CHAT_DATA_DIR", "/data")
	pnmURL := env("CHAT_PNM_URL", "https://pnm.cloudsoftphone.com/pnm2/send")

	st, err := store.Open(dataDir)
	if err != nil {
		log.Fatalf("store: %v", err)
	}
	router, err := push.NewRouter(st, pnmURL)
	if err != nil {
		log.Fatalf("push: %v", err)
	}
	mw := api.NewMiddleware(mwURL)
	srv := &api.Server{Host: host, C2SAddr: c2s, PushService: "push." + host, VAPIDPublic: router.VAPIDPublic, Store: st, MW: mw, EjabberdAPI: env("CHAT_EJABBERD_API", "http://127.0.0.1:5280/api"), UploadURL: os.Getenv("CHAT_UPLOAD_URL")}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	router.RoomName = push.RoomName
	router.Names = mw.Name
	router.Address = mw.Extension
	router.ArchiveID = func(ctx context.Context, user, peer string) string {
		ses, err := xmppc.Dial(ctx, c2s, host, user, svcauth.Password(secret, user, host, time.Now()))
		if err != nil {
			log.Printf("push: archive id for %s: %v", user, err)
			return ""
		}
		defer ses.Close()
		m, err := ses.Last(ctx, peer)
		if err != nil {
			log.Printf("push: archive id for %s with %s: %v", user, peer, err)
			return ""
		}
		return m.ID
	}
	go push.ServeComponent(ctx, componentAddr, host, secret, router)

	hs := &http.Server{Addr: listen, Handler: srv.Handler(), ReadHeaderTimeout: 10 * time.Second}
	go func() {
		log.Printf("chat-gateway: listening on %s for %s", listen, host)
		if err := hs.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("http: %v", err)
		}
	}()
	<-ctx.Done()
	shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = hs.Shutdown(shutdown)
}
