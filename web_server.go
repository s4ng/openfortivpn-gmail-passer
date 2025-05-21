package main

import (
	"context"
	"fmt"
	"golang.org/x/oauth2"
	"log"
	"net/http"
)

func openServerAndSaveToken(config *oauth2.Config, tokenPath string) {
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
		fmt.Println("✅ 토큰 저장에 성공하였습니다. 프로그램을 종료하고 다시 시작해주세요.")
	})
	err := http.ListenAndServe(":80", nil)
	if err != nil {
		return
	}
}
