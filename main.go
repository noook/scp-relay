package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/bramvdbogaerde/go-scp"
	"github.com/bramvdbogaerde/go-scp/auth"

	"github.com/gorilla/mux"
)

var chars = generatePossibleChars()

type CredentialsData struct {
	Username   string `json:"username"`
	Password   string `json:"password"`
	Host       string `json:"host"`
	RemotePath string `json:"remotePath"`
}

func main() {
	router := mux.NewRouter().StrictSlash(true)
	router.HandleFunc("/credentials", CredentialsHandler)
	router.HandleFunc("/file", fileHandler)
	log.Fatal(http.ListenAndServe(":8787", router))
}

func generatePossibleChars() (list []rune) {
	for i := 48; i <= 57; i++ {
		list = append(list, rune(i))
	}
	for i := 65; i <= 90; i++ {
		list = append(list, rune(i))
	}
	for i := 97; i <= 122; i++ {
		list = append(list, rune(i))
	}

	return list
}

func HandleFile(r *http.Request) (link string, err error) {
	r.ParseMultipartForm(32 << 20)

	username := r.MultipartForm.Value["username"][0]
	password := r.MultipartForm.Value["password"][0]
	host := r.MultipartForm.Value["host"][0]
	remotePath := r.MultipartForm.Value["remotePath"][0]
	remoteURL := r.MultipartForm.Value["remoteUrl"][0]

	id := Guid()
	for !IsAvailable(remoteURL, id, "png") {
		id = Guid()
	}

	file, _, err := r.FormFile("file")
	defer file.Close()

	if err != nil {
		return "", err
	}

	f, err := os.OpenFile("./images/"+id+".png", os.O_WRONLY|os.O_CREATE, 0666)

	if err != nil {
		return "", err
	}

	io.Copy(f, file)

	UploadFile(id, username, password, host, remotePath)

	link = fmt.Sprintf("%s%s.png", SanitizePath(remoteURL), id)

	os.Remove("./images/" + id + ".png")

	return link, nil
}

func UploadFile(id string, username string, password string, host string, path string) {
	clientConfig, _ := auth.PasswordKey(username, password, ssh.InsecureIgnoreHostKey())
	client := scp.NewClient(host+":22", &clientConfig)

	err := client.Connect()

	if err != nil {
		log.Fatal(err)
	}

	defer client.Close()

	filepath := "images/" + id + ".png"
	file, _ := os.Open(filepath)
	stat, _ := file.Stat()

	err = client.CopyFile(file, SanitizePath(path)+stat.Name(), "0644")
}

func Guid() (identifier string) {
	rand.Seed(time.Now().UTC().UnixNano())
	for i := 0; i < 7; i++ {
		identifier += string(chars[rand.Intn(len(chars))])
	}
	return identifier
}

func IsAvailable(rootpath string, id string, ext string) bool {
	url := fmt.Sprintf("%s%s.%s", SanitizePath(rootpath), id, ext)
	resp, err := http.Get(url)
	if err != nil {
		log.Fatal(err)
	}

	return resp.StatusCode == 404
}

func fileHandler(w http.ResponseWriter, r *http.Request) {
	link, err := HandleFile(r)
	if err != nil {
		log.Fatal("Error while writing file")
	}

	io.WriteString(w, fmt.Sprintf(`{"link": "%s"}`, link))
}

// func UploadFile(filename string)

func CredentialsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var data CredentialsData

	reqBody, err := ioutil.ReadAll(r.Body)
	if err != nil {
		fmt.Fprintf(w, "Error while reading data")
	}

	json.Unmarshal(reqBody, &data)

	clientConfig, _ := auth.PasswordKey(data.Username, data.Password, ssh.InsecureIgnoreHostKey())
	client := scp.NewClient(data.Host+":22", &clientConfig)

	err = client.Connect()
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		io.WriteString(w, `{"error": "Invalid credentials"}`)
	} else {
		file, _ := os.Open("resources/write-test.txt")
		stat, _ := file.Stat()

		err = client.CopyFile(file, SanitizePath(data.RemotePath)+stat.Name(), "0644")
		defer client.Close()

		if err != nil {
			fmt.Println(err)
			w.WriteHeader(http.StatusBadRequest)
			io.WriteString(w, `{"error": "Invalid path"}`)
		} else {
			io.WriteString(w, `{}`)
		}
	}
}

func SanitizePath(path string) string {
	if path[len(path)-1:] != "/" {
		path = path + "/"
	}

	return path
}
