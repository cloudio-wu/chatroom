package util

import "log"

func logFatalIfNull(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
