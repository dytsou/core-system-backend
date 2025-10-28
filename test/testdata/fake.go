package testdata

import (
	"github.com/brianvoe/gofakeit/v7"
)

func RandomEmail() string {
	return gofakeit.Email()
}

func RandomFullName() string {
	return gofakeit.Name()
}

func RandomName() string {
	return gofakeit.Sentence(1)
}

func RandomDescription() string {
	return gofakeit.Sentence(10)
}

func RandomURL() string {
	return gofakeit.URL()
}

func RandomSlug() string {
	return gofakeit.Sentence(1)
}
