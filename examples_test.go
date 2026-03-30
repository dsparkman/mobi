package mobi_test

import (
	"log"
	"math/rand/v2"
	"os"

	"github.com/dsparkman/mobi"
	"golang.org/x/text/language"
)

func ExampleBook() {
	ch := mobi.SimpleChapter("Chapter 1", "<p>Lorem ipsum dolor sit amet, consetetur sadipscing elitr.</p>")

	book, err := mobi.NewBook("De vita Caesarum librus",
		rand.Uint32(),
		mobi.WithAuthors("Sueton"),
		mobi.WithLanguage(language.Italian),
		mobi.WithChapters(ch),
	)
	if err != nil {
		log.Fatal(err)
	}

	db, err := book.Realize()
	if err != nil {
		log.Fatal(err)
	}

	f, err := os.Create("test.azw3")
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = f.Close() }()

	if err := db.Write(f); err != nil {
		log.Fatal(err)
	}
}
