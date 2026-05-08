package entropy

func Drift(current, pressure float64) float64 {
	next := current + pressure
	if next < 0 {
		return 0
	}
	if next > 1 {
		return 1
	}
	return next
}

func ArchiveIntegrity(current, entropyDelta float64) float64 {
	next := current - entropyDelta
	if next < 0 {
		return 0
	}
	if next > 1 {
		return 1
	}
	return next
}
