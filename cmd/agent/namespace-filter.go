package main

func isDetectableNamespace(ns string) bool {
	switch ns {
	case "kube-system",
		"kube-public",
		"kube-node-lease",
		"flowmancer-system":
		return false
	default:
		return true
	}
}
