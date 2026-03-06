/**
##
## OverDrive 2026
## All Technical rights reserved
##
## database.go - File to initialize the PostgreSQL db client with prisma ORM.
##
*/

package database

import (
	"context"
	"overdrive/resources/db"
)

var PrismaClient *db.PrismaClient

func Connect() error {
	PrismaClient = db.NewClient()
	if err := PrismaClient.Prisma.Connect(); err != nil {
		return err
	}
	return nil
}

func Disconnect() error {
	if err := PrismaClient.Prisma.Disconnect(); err != nil {
		return err
	}
	return nil
}

func IsConnected() bool {
	if PrismaClient == nil || PrismaClient.Prisma == nil {
		return false
	}

	var result []map[string]interface{}
	if err := PrismaClient.Prisma.QueryRaw(`SELECT 1`).Exec(context.Background(), &result); err != nil {
		return false
	}

	return true
}
