package main

import (
	"log"

	"github.com/basvdlei/gazcli/internal/cli"
)

func main() {
	if err := cli.NewGazcli(); err != nil {
		log.Fatalln(err)
	}
}
