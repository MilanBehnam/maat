package main

import "testing"

func TestCreateServerPool(t *testing.T) {
	servers := []string{"http://localhost.com", "example.com"}
	createServerPool(&servers)
	if len(serverPool.Backends) == 2 {
		t.Log("two servers added to servers pool.")
	} else {
		t.Error("servers weren't added to server pool.")
	}
}
