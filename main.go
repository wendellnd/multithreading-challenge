package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/wendellnd/multithreading-challenge/address"
)

func main() {
	cep := "01001000"
	ctx := context.Background()

	addressService := address.NewAddressService(ctx)
	addressService.SetTimeout(1 * time.Second)

	result, err := addressService.Execute(cep)
	if err != nil {
		log.Println(err.Error())
		return
	}

	fmt.Printf("%+v", result)
}
