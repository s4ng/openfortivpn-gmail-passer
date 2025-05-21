package main

import (
	"encoding/json"
	"fmt"
	"golang.org/x/oauth2"
	"log"
	"os"
)

// Retrieves a token from a local file.
func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

// Saves a token to a file path.
func saveToken(path string, token *oauth2.Token) {
	fmt.Printf("Saving credential file to: %s\n", path)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Fatalf("Unable to cache oauth token: %v", err)
	}
	defer f.Close()
	json.NewEncoder(f).Encode(token)
}

func removeTokenFile() {
	tokFile := "token.json"
	tokPath := basePath + tokFile
	err := os.Remove(tokPath)
	if err != nil {
		log.Fatalf("Unable to remove token file: %v", err)
		return
	}
}

func createDirectory(path string) error {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		// 디렉토리가 없으면 생성
		err := os.MkdirAll(path, 0755)
		if err != nil {
			return fmt.Errorf("디렉토리 생성 실패: %w", err)
		}
		fmt.Println("📁 디렉토리를 생성했습니다:", path)
	} else if err != nil {
		return fmt.Errorf("디렉토리 확인 중 오류 발생: %w", err)
	} else if !info.IsDir() {
		return fmt.Errorf("해당 경로는 디렉토리가 아닙니다: %s", path)
	}
	return nil
}
