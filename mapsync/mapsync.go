package mapsync

import "sync"

type MapSync struct {
	mapSync sync.Map
}

func NewMapSync() *MapSync{
	node := &MapSync{}
	return node
}

func (this *MapSync)Store(key, value interface{}){
	 this.mapSync.Store(key,value)
}

func (this *MapSync)LoadOrStore(key, value interface{})( interface{},  bool){
	return this.mapSync.LoadOrStore(key,value)
}

func (this *MapSync)Load(key interface{})(interface{},  bool){
	return this.mapSync.Load(key)
}

func (this *MapSync)Delete(key interface{}){
	 this.mapSync.Delete(key)
}

func (this *MapSync)Clear(){
	box := make([]interface{},0,100)
	this.mapSync.Range(func(k, v interface{}) bool {
		box = append(box, k)
		//this.mapSync.Delete(k)
		return true
	})
	for _,k := range box{
		this.mapSync.Delete(k)
	}
}

func (this *MapSync)Len() int{
	i := 0
	this.mapSync.Range(func(k, v interface{}) bool {
		i++
		return true
	})
	return i
}

func (this *MapSync)Range(f func(key, value interface{}) bool) {
	this.mapSync.Range(f)
}