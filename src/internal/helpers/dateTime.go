package helpers

func GenerateDateNow() string {
	return Now().Format("2006-01-02T15:04:05Z")
}

func GenerateDateNowMs() string {
	return Now().Format("2006-01-02T15:04:05.000Z")
}
