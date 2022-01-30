package main

func init() {
	for i := 1; i <= 10; i++ {
		go syncWorker()
	}

	for i := 1; i <= 2; i++ {
		go addWorker()
	}

	for i := 1; i <= 1; i++ {
		go syncAuthorWorker()
	}
}
