//go:build !darwin && !windows

package flowengine

import "dkst-text-flow/internal/storage"

type Store interface{}

func Start(store Store) bool                            { return false }
func Stop()                                             {}
func Running() bool                                     { return false }
func SetExpansionHandler(handler func(storage.Snippet)) {}
