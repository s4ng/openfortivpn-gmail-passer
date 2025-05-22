package main

import (
	"context"
	"golang.org/x/oauth2"
	"log"
	"net/http"
	"sync"
)

func openServerAndSaveToken(wg *sync.WaitGroup, config *oauth2.Config, tokenPath string) *http.Server {

	srv := &http.Server{Addr: ":80"}

	http.HandleFunc("/", func(rw http.ResponseWriter, r *http.Request) {
		authCode := r.FormValue("code")
		if authCode == "" {
			return
		}
		_, err := rw.Write([]byte("Success!!\n"))
		if err != nil {
			return
		}
		token, err := config.Exchange(context.TODO(), authCode)
		if err != nil {
			log.Fatalf("Unable to retrieve token from web: %v", err)
		}
		saveToken(tokenPath, token)
	})

	go func() {
		defer wg.Done() // let main know we are done cleaning up

		// always returns error. ErrServerClosed on graceful close
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			// unexpected error. port in use?
			log.Fatalf("ListenAndServe(): %v", err)
		}
	}()

	return srv
}

func closeServer(srv *http.Server, wg *sync.WaitGroup) {

	if err := srv.Shutdown(context.TODO()); err != nil {
		panic(err)
	}

	wg.Wait()
}
