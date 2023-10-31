package main

import (
	"context"
	"fmt"
	"log"

	"github.com/Ja7ad/irys"
	"github.com/Ja7ad/irys/configs"
	"github.com/Ja7ad/irys/currency"
)

func main() {
	matic, err := currency.NewMatic(configs.ExamplePrivateKey, configs.ExampleRpc)
	if err != nil {
		log.Fatal(err)
	}

	c, err := irys.New(irys.DefaultNode2, matic, false)
	if err != nil {
		log.Fatal(err)
	}

	tx, err := c.GetMetaData(context.Background(), "XjzDyneweD_Dmhuaipbi7HyXXvsY6IkMcIsumlB0G2M")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(tx)
}
