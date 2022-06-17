package main

import "errors"

var (
	errUnsupportChain  = errors.New("unsupport chain")
	errUnsupportToken  = errors.New("unsupport token")
	errUnsupportMethod = errors.New("unsupport method")
)
