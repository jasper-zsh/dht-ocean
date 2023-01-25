package util

func EmptyChannel[T interface{}](ch chan T) {
	for len(ch) > 0 {
		<-ch
	}
}
