package conc

import "sync"

type List[T any] struct {
	list []T

	rwLock sync.RWMutex
}

func NewList[T any]() *List[T] {
	return &List[T]{}
}

func (l *List[T]) Push(value T) (index int) {
	l.rwLock.Lock()
	defer l.rwLock.Unlock()

	l.list = append(l.list, value)
	return len(l.list) - 1
}

func (l *List[T]) Len() int {
	l.rwLock.RLock()
	defer l.rwLock.RUnlock()

	return len(l.list)
}

func (l *List[T]) Get(index int) T {
	l.rwLock.RLock()
	defer l.rwLock.RUnlock()

	return l.list[index]
}

func (l *List[T]) Remove(index int) {
	l.rwLock.Lock()
	defer l.rwLock.Unlock()

	l.list = append(l.list[:index], l.list[index+1:]...)
}

func (l *List[T]) ForEach(cb func(value T) bool) {
	l.rwLock.RLock()
	defer l.rwLock.RUnlock()

	for _, value := range l.list {
		if !cb(value) {
			break
		}
	}
}
