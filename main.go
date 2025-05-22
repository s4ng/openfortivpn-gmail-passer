package main

import (
	"context"
	"fmt"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/oauth2"
	"google.golang.org/api/gmail/v1"
)

var basePath string

func init() {
	basePath = getOriginalUserHome() + "/.config/gvpn/"
}

func getOriginalUserHome() string {
	// SUDO_USER가 설정되어 있으면 원래 사용자로 추정
	sudoUser := os.Getenv("SUDO_USER")
	if sudoUser == "" {
		return os.Getenv("HOME") // 일반 사용자 실행
	}

	// 원래 사용자의 홈 디렉토리를 쉘에서 얻어옴
	out, err := exec.Command("sh", "-c", "eval echo ~"+sudoUser).Output()
	if err != nil {
		log.Printf("⚠️  원래 사용자 홈 디렉토리 가져오기 실패, fallback to HOME: %v", err)
		return os.Getenv("HOME")
	}
	return strings.TrimSpace(string(out))
}

// Retrieve a token, saves the token, then returns the generated client.
func getClient(config *oauth2.Config) *http.Client {
	// The file token.json stores the user's access and refresh tokens, and is
	// created automatically when the authorization flow completes for the first
	// time.
	tokenFile := "token.json"
	tokenPath := basePath + tokenFile
	tok, err := tokenFromFile(tokenPath)
	if err != nil {

		httpServerExitDone := &sync.WaitGroup{}
		httpServerExitDone.Add(1)

		srv := saveTokenFromWeb(httpServerExitDone, config, tokenPath)
		// 1분간 OAuth 인증 대기
		for i := 0; i < 60; i++ {
			if tmpTok, _ := tokenFromFile(tokenPath); tmpTok != nil {
				break
			}
			time.Sleep(time.Duration(1) * time.Second)
		}
		tok, err = tokenFromFile(tokenPath)
		closeServer(srv, httpServerExitDone)
	}
	return config.Client(context.Background(), tok)
}

// Request a token from the web, then returns the retrieved token.
func saveTokenFromWeb(wg *sync.WaitGroup, config *oauth2.Config, tokenPath string) *http.Server {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("🔐 OAuth 토큰이 필요합니다. 아래 링크에 접속해주세요."+
		"\n%v\n", authURL)
	return openServerAndSaveToken(wg, config, tokenPath)
}

func removeReadMail(srv *gmail.Service, user string, messageId string) {
	_, err := srv.Users.Messages.Trash(user, messageId).Do()
	if err == nil {
		fmt.Printf("✉️ Message %s removed.\n", messageId)
	} else {
		fmt.Printf("❌ Message %s remove failed.\n%s\n", messageId, err)
	}
}

func getGmailService() (*gmail.Service, error) {
	ctx := context.Background()
	_ = createDirectory(basePath)

	b, err := os.ReadFile(basePath + "credentials.json")
	if err != nil {
		log.Fatalf("Unable to read client secret file: %v", err)
	}

	// If modifying these scopes, delete your previously saved token.json.
	config, err := google.ConfigFromJSON(b, gmail.GmailReadonlyScope, gmail.GmailModifyScope)
	if err != nil {
		log.Fatalf("Unable to parse client secret file to config: %v", err)
	}
	client := getClient(config)

	return gmail.NewService(ctx, option.WithHTTPClient(client))
}

func waitEmailAndVerify(srv *gmail.Service, user string, lastMsgDate string) (messageId string, authCode string, err error) {
	// 10회 실행
	for i := 0; i < 10; i++ {
		// 최근 이메일 5개 조회
		msgList, err := srv.Users.Messages.List(user).MaxResults(3).LabelIds("INBOX").Do()
		if err != nil {
			log.Fatalf("Unable to retrieve messages: %v", err)
		}

		if len(msgList.Messages) == 0 {
			fmt.Println("No messages found.")
			return "", "", err
		}

		for _, m := range msgList.Messages {
			msg, err := srv.Users.Messages.Get(user, m.Id).Format("metadata").MetadataHeaders("Subject", "Date").Do()
			if err != nil {
				log.Printf("Unable to get message %s: %v", m.Id, err)
				continue
			}

			subject := "N/A"
			date := "N/A"
			for _, header := range msg.Payload.Headers {
				switch header.Name {
				case "Subject":
					subject = header.Value
				case "Date":
					date = header.Value
				}
			}

			lastMsgDateTime, err := parseDate(lastMsgDate)
			if err != nil {
				fmt.Println("Error parsing time:", err)
				return "", "", err
			}
			dateTime, err := parseDate(date)
			if err != nil {
				fmt.Println("Error parsing time:", err)
				return "", "", err
			}
			if dateTime.Equal(lastMsgDateTime) || dateTime.Before(lastMsgDateTime) {
				continue
			}

			if strings.Contains(subject, "AuthCode") {

				// Auth code 반환
				return m.Id, subject[10:16], nil
			}
		}
		time.Sleep(time.Duration(1) * time.Second)
	}
	return "", "", fmt.Errorf("email verification timeout")
}

func connectVpnWithEmailVerification() {
	srv, err := getGmailService()
	if err != nil {
		log.Fatalf("Unable to get Gmail service: %v", err)
	}

	user := "me"

	// check last mail date
	lastMsgList, err := srv.Users.Messages.List(user).MaxResults(1).LabelIds("INBOX").Do()
	lastMsgDate := "N/A"

	if err != nil {
		if strings.Contains(err.Error(), "invalid_grant") {
			removeTokenFile()
			log.Fatalf("🚫 OAuth 토큰이 만료되었습니다. 다시 시도해주세요.")
			return
		}
		log.Fatalf("Unable to retrieve messages: %v", err)
		return
	}

	for _, m := range lastMsgList.Messages {
		msg, err := srv.Users.Messages.Get(user, m.Id).Format("metadata").MetadataHeaders("Date").Do()
		if err != nil {
			log.Printf("Unable to get message %s: %v", m.Id, err)
			return
		}

		for _, header := range msg.Payload.Headers {
			switch header.Name {
			case "Date":
				lastMsgDate = header.Value
			}
		}
	}

	// start process
	app := "openfortivpn"
	arg0 := "-c"
	arg1 := basePath + "config"
	cmd := exec.Command(app, arg0, arg1)

	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	// 표준 출력/에러도 보고 싶다면 아래처럼 연결

	stdin, _ := cmd.StdinPipe()
	stdoutPipe, _ := cmd.StdoutPipe()
	stderrPipe, _ := cmd.StderrPipe()

	// 프로세스 시작
	if err := cmd.Start(); err != nil {
		log.Fatalf("Failed to start command: %v", err)
	}
	// stdout/stderr 읽기
	go io.Copy(os.Stdout, stdoutPipe)
	go io.Copy(os.Stderr, stderrPipe)

	// 시그널 처리
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigs
		fmt.Printf("\n🚨 Caught signal: %v\n", sig)
		pgid, _ := syscall.Getpgid(cmd.Process.Pid)
		syscall.Kill(-pgid, sig.(syscall.Signal))
		os.Exit(1)
	}()

	fmt.Println("⏳ Waiting for email verification...")

	messageId, authCode, err := waitEmailAndVerify(srv, user, lastMsgDate)
	if err != nil {
		log.Printf("Email verification failed: %v", err)
		return
	}
	removeReadMail(srv, user, messageId)

	fmt.Println("code : ", authCode)
	fmt.Println("✅ Email verification complete.")
	go func() {
		defer stdin.Close()
		io.WriteString(stdin, authCode+"\n")
	}()

	if err := cmd.Wait(); err != nil {
		log.Printf("VPN process exited with error: %v", err)
	} else {
		fmt.Println("✅ VPN process complete.")
	}

	return
}

func main() {
	connectVpnWithEmailVerification()
}
