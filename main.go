package main

import (
  "context"
  "encoding/json"
  "fmt"
  "log"
  "net/http"
  "io"
  "os"
  "os/exec"
  "os/signal"
  "syscall"
  "time"
  "strings"

  "golang.org/x/oauth2"
  "golang.org/x/oauth2/google"
  "google.golang.org/api/gmail/v1"
  "google.golang.org/api/option"
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
  tokFile :=  "token.json"
  tokPath := basePath + tokFile
  tok, err := tokenFromFile(tokPath)
  if err != nil {
    tok = getTokenFromWeb(config)
    saveToken(tokPath, tok)
  }
  return config.Client(context.Background(), tok)
}

// Request a token from the web, then returns the retrieved token.
func getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
  authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
  fmt.Printf("Go to the following link in your browser then type the "+
    "authorization code: \n%v\n", authURL)

  var authCode string
  if _, err := fmt.Scan(&authCode); err != nil {
    log.Fatalf("Unable to read authorization code: %v", err)
  }

  tok, err := config.Exchange(context.TODO(), authCode)
  if err != nil {
    log.Fatalf("Unable to retrieve token from web: %v", err)
  }
  return tok
}

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

func main() {
  ctx := context.Background()
  createDirectory(basePath)

  b, err := os.ReadFile(basePath + "credentials.json")
  if err != nil {
    log.Fatalf("Unable to read client secret file: %v", err)
  }

  // If modifying these scopes, delete your previously saved token.json.
  config, err := google.ConfigFromJSON(b, gmail.GmailReadonlyScope)
  if err != nil {
    log.Fatalf("Unable to parse client secret file to config: %v", err)
  }
  client := getClient(config)

  srv, err := gmail.NewService(ctx, option.WithHTTPClient(client))
  if err != nil {
    log.Fatalf("Unable to retrieve Gmail client: %v", err)
  }

  user := "me"

  // check last mail date
  lastMsgList, err := srv.Users.Messages.List(user).MaxResults(1).LabelIds("INBOX").Do()
  lastMsgDate := "N/A"

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

  // 10회 실행
  for i := 0; i < 10; i++ {
    // 최근 이메일 5개 조회
    msgList, err := srv.Users.Messages.List(user).MaxResults(3).LabelIds("INBOX").Do()
    if err != nil {
      log.Fatalf("Unable to retrieve messages: %v", err)
    }

    if len(msgList.Messages) == 0 {
      fmt.Println("No messages found.")
      return
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

      lastMsgDateTime, err := time.Parse(time.RFC1123Z, lastMsgDate)
      if err != nil {
        fmt.Println("Error parsing time:", err)
        return
      }
      dateTime, err := time.Parse(time.RFC1123Z, date)
      if err != nil {
        fmt.Println("Error parsing time:", err)
        return
      }
      if dateTime.Equal(lastMsgDateTime) || dateTime.Before(lastMsgDateTime) {
        continue
      }

      if strings.Contains(subject, "AuthCode") {

        code := subject[10:16]
        fmt.Println("code : %s", code)
        fmt.Println("✅ Email verification complete.")
        go func() {
          defer stdin.Close()
          io.WriteString(stdin, code + "\n")
        }()
        // stdout/stderr 읽기
        go io.Copy(os.Stdout, stdoutPipe)
        go io.Copy(os.Stderr, stderrPipe)

        if err := cmd.Wait(); err != nil {
          log.Printf("VPN process exited with error: %v", err)
        } else {
          fmt.Println("✅ VPN process complete.")
        }

        return
      }
    }
    time.Sleep(time.Duration(1) * time.Second)
  }
}
