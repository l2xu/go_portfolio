package main

import (
	"context"
	"flag"
	"fmt"
	"html/template"
	"io"

	// "io/ioutil"
	"log"
	"net/http"

	"path/filepath"
	"time"

	// "github.com/russross/blackfriday/v2"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

var (
	srcDir = flag.String("src", "./projects", "Inhalte -Dir.")
	tmpDir = flag.String("tmp", "./templates", "Template -Dir.")
)

// All the data that is needed for the template
type Page struct {
	Title       string
	Short       string
	Image_url   string
	Description string
	Date        string
}

type Pages []Page

var ps Pages

func loadContent() {

	//CHECK FOR ENV VARIABLE
	// mongoURI, ok := os.LookupEnv("MONGO_URI")
	// if !ok {
	// 	log.Fatal("MONGO_URI environment variable not set")
	// }

	ctx, cancel := context.WithTimeout(
		context.Background(), 10*time.Second)
	defer cancel()
	//replace with mongoUIR string
	opt := options.Client().ApplyURI("mongodb://127.0.0.1:21017/")
	client, err := mongo.Connect(ctx, opt)
	if err != nil {
		log.Fatal(err)
	}
	// check for connection
	if err := client.Ping(ctx, readpref.Primary()); err != nil {
		log.Fatal(err)
	}

	myProjects := client.Database("go_portfolio").Collection("projects")

	// Find all projects
	cursor, err := myProjects.Find(context.TODO(), bson.M{})
	if err != nil {
		log.Fatal(err)
	}

	// Iterate through the cursor and print the documents
	var projects []bson.M
	if err = cursor.All(context.TODO(), &projects); err != nil {
		log.Fatal(err)
	}
	for _, project := range projects {
		page := Page{}
		title, ok := project["title"]
		short, ok := project["short"]
		image_url, ok := project["image_url"]
		description, ok := project["description"]
		date, ok := project["date"]

		if ok {
			page.Title = title.(string)
			page.Short = short.(string)
			page.Image_url = image_url.(string)
			page.Description = description.(string)
			page.Date = date.(string)
			ps = append(ps, page)
		}
	}
}

// Start the server and handle the requests
// Sets the directory for static files
func main() {

	loadContent()

	flag.Parse()
	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	http.HandleFunc("/", makeIndexHandler())
	http.HandleFunc("/projects/", makePageHandler())

	//start the server
	log.Print("Listening on :9000 ....")
	err := http.ListenAndServe(":9000", nil)
	if err != nil {
		log.Fatal(err)
	}

}

// Here the index page is created with the index.templ.html
func makeIndexHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// ps, err := loadPages()
		// if err != nil {
		// 	log.Println(err)
		// }
		err := renderPage(w, ps, "index.templ.html")
		if err != nil {
			log.Println(err)
		}

	}
}

// Here the project page is created with the project.templ.html
func makePageHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		//get the url and split it to get the project name
		url := r.URL.Path
		url = url[10:]

		p, err := loadPage(url)
		fmt.Print(p)
		if err != nil {
			log.Println(err)
		}
		err = renderPage(w, p, "project.templ.html")
		if err != nil {
			log.Println(err)
		}
	}
}

// Here the template page is rendered
func renderPage(w io.Writer, data interface{}, content string) error {
	temp, err := template.ParseFiles(
		filepath.Join(*tmpDir, "base.templ.html"),
		filepath.Join(*tmpDir, content),
	)

	if err != nil {
		return fmt.Errorf("renderPage.Parsefiles: %w", err)
	}
	err = temp.ExecuteTemplate(w, "base", data)

	if err != nil {
		return fmt.Errorf("renderPage.ExecuteTemplate: %w", err)
	}
	return nil
}

// here the struct get filled with data
func loadPage(url string) (Page, error) {
	var p Page
	fmt.Println(url)
	fmt.Print(ps)

	//search for the project in the struct
	for _, page := range ps {
		if page.Title == url {
			p.Title = page.Title
			p.Description = page.Description
			p.Image_url = page.Image_url
			p.Short = page.Short
			p.Date = page.Date

		}
	}

	return p, nil
}
