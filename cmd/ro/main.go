package main

import (
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"time"

	"github.com/cloudflare/redoctober/client"
	"github.com/cloudflare/redoctober/cmd/ro/gopass"
	"github.com/cloudflare/redoctober/core"
	"github.com/cloudflare/redoctober/order"
)

var action, user, pswd, userEnv, pswdEnv, server, caPath string

var owners, lefters, righters, inPath, labels, outPath, outEnv string

var uses int

var duration, users string

var pollInterval time.Duration

type command struct {
	Run  func()
	Desc string
}

var roServer *client.RemoteServer

var commandSet = map[string]command{
	"create":     command{Run: runCreate, Desc: "create a user account"},
	"summary":    command{Run: runSummary, Desc: "list the user and delegation summary"},
	"delegate":   command{Run: runDelegate, Desc: "do decryption delegation"},
	"encrypt":    command{Run: runEncrypt, Desc: "encrypt a file"},
	"decrypt":    command{Run: runDecrypt, Desc: "decrypt a file"},
	"re-encrypt": command{Run: runReEncrypt, Desc: "re-encrypt a file"},
	"order":      command{Run: runOrder, Desc: "place an order for delegations"},
}

func registerFlags() {
	flag.StringVar(&server, "server", "localhost:8080", "server address")
	flag.StringVar(&caPath, "ca", "", "ca file path")
	flag.StringVar(&owners, "owners", "", "comma separated owner list")
	flag.StringVar(&users, "users", "", "comma separated user list")
	flag.IntVar(&uses, "uses", 0, "number of delegated key uses")
	flag.StringVar(&duration, "time", "0h", "duration of delegated key uses")
	flag.StringVar(&lefters, "left", "", "comma separated left owners")
	flag.StringVar(&righters, "right", "", "comma separated right owners")
	flag.StringVar(&labels, "labels", "", "comma separated labels")
	flag.StringVar(&inPath, "in", "", "input data file")
	flag.StringVar(&outPath, "out", "", "output data file")
	flag.StringVar(&outEnv, "outenv", "", "env variable for output data")
	flag.StringVar(&user, "user", "", "username")
	flag.StringVar(&pswd, "password", "", "password")
	flag.StringVar(&userEnv, "userenv", "RO_USER", "env variable for user name")
	flag.StringVar(&pswdEnv, "pswdenv", "RO_PASS", "env variable for user password")
	flag.DurationVar(&pollInterval, "poll-interval", time.Second, "interval for polling an outstanding order (set 0 to disable polling)")
}

func getUserCredentials() {
	user = os.Getenv(userEnv)
	pswd = os.Getenv(pswdEnv)
	if user == "" || pswd == "" {
		fmt.Print("Username:")
		fmt.Scan(&user)
		var err error
		pswd, err = gopass.GetPass("Password:")
		processError(err)
	}
}

func processError(err error) {
	if err != nil {
		log.Fatal("error:", err)
	}
}

func processCSL(s string) []string {
	if s == "" {
		return nil
	}

	return strings.Split(s, ",")
}

func runCreate() {
	req := core.CreateRequest{
		Name:     user,
		Password: pswd,
	}
	resp, err := roServer.Create(req)
	processError(err)
	fmt.Println(resp.Status)
}

func runDelegate() {
	req := core.DelegateRequest{
		Name:     user,
		Password: pswd,
		Uses:     uses,
		Time:     duration,
		Users:    processCSL(users),
		Labels:   processCSL(labels),
	}
	resp, err := roServer.Delegate(req)
	processError(err)
	fmt.Println(resp.Status)
}

// TODO: summary response needs better formatting
func runSummary() {
	req := core.SummaryRequest{
		Name:     user,
		Password: pswd,
	}
	resp, err := roServer.Summary(req)
	processError(err)
	fmt.Println(resp)
}

func runEncrypt() {
	inBytes, err := ioutil.ReadFile(inPath)
	processError(err)
	req := core.EncryptRequest{
		Name:        user,
		Password:    pswd,
		Owners:      processCSL(owners),
		LeftOwners:  processCSL(lefters),
		RightOwners: processCSL(righters),
		Labels:      processCSL(labels),
		Data:        inBytes,
	}

	resp, err := roServer.Encrypt(req)
	processError(err)
	if resp.Status != "ok" {
		log.Fatal("response status error:", resp.Status)
		return
	}
	fmt.Println("Response Status:", resp.Status)
	outBytes := []byte(base64.StdEncoding.EncodeToString(resp.Response))
	ioutil.WriteFile(outPath, outBytes, 0644)
}

func runReEncrypt() {
	inBytes, err := ioutil.ReadFile(inPath)
	processError(err)

	// base64 decode the input
	encBytes, err := base64.StdEncoding.DecodeString(string(inBytes))
	if err != nil {
		log.Println("fail to base64 decode the data, proceed with raw data")
		encBytes = inBytes
	}

	req := core.ReEncryptRequest{
		Name:        user,
		Password:    pswd,
		Owners:      processCSL(owners),
		LeftOwners:  processCSL(lefters),
		RightOwners: processCSL(righters),
		Labels:      processCSL(labels),
		Data:        encBytes,
	}

	resp, err := roServer.ReEncrypt(req)
	processError(err)
	if resp.Status != "ok" {
		log.Fatal("response status error:", resp.Status)
		return
	}
	fmt.Println("Response Status:", resp.Status)
	outBytes := []byte(base64.StdEncoding.EncodeToString(resp.Response))
	ioutil.WriteFile(outPath, outBytes, 0644)
}

func runDecrypt() {
	inBytes, err := ioutil.ReadFile(inPath)
	processError(err)

	// base64 decode the input
	encBytes, err := base64.StdEncoding.DecodeString(string(inBytes))
	if err != nil {
		log.Println("fail to base64 decode the data, proceed with raw data")
		encBytes = inBytes
	}

	req := core.DecryptRequest{
		Name:     user,
		Password: pswd,
		Data:     encBytes,
	}

	resp, err := roServer.Decrypt(req)
	processError(err)
	if resp.Status != "ok" {
		log.Fatal("response status error:", resp.Status)
		return
	}
	fmt.Println("Response Status:", resp.Status)
	var msg core.DecryptWithDelegates
	err = json.Unmarshal(resp.Response, &msg)
	processError(err)
	fmt.Println("Secure:", msg.Secure)
	fmt.Println("Delegates:", msg.Delegates)
	ioutil.WriteFile(outPath, msg.Data, 0644)
}

func runOrder() {
	req := core.OrderRequest{
		Name:     user,
		Password: pswd,
		Uses:     uses,
		Duration: duration,
		Labels:   processCSL(labels),
		Users:    processCSL(users),
	}
	resp, err := roServer.Order(req)
	processError(err)

	var o order.Order
	err = json.Unmarshal(resp.Response, &o)
	processError(err)

	if pollInterval > 0 {
		for o.Delegated < 2 {
			time.Sleep(pollInterval)
			resp, err = roServer.OrderInfo(core.OrderInfoRequest{Name: user, Password: pswd, OrderNum: o.Num})
			processError(err)
			err = json.Unmarshal(resp.Response, &o)
			processError(err)
		}
	}
	fmt.Println(resp.Status)
}

func main() {
	flag.Usage = func() {
		fmt.Println("Usage: ro [options] subcommand")
		fmt.Println("Currently supported subcommands are:")
		for key := range commandSet {
			fmt.Println("\t", key, ":", commandSet[key].Desc)
		}

		fmt.Println("Options:")
		flag.PrintDefaults()
	}

	registerFlags()
	flag.Parse()

	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(1)
	}

	action := flag.Arg(0)

	cmd, found := commandSet[action]
	if !found {
		fmt.Println("Unsupported subcommand:", action)
		flag.Usage()
		os.Exit(1)
	} else {
		var err error
		roServer, err = client.NewRemoteServer(server, caPath)
		processError(err)

		getUserCredentials()
		cmd.Run()
	}
}
