package merkledag

import (
	"encoding/json"
	"hash"
)

type Link struct {
	Name string
	Hash []byte
	Size int
}

type Object struct {
	Links []Link
	Data  []byte
}

const SIZE = 256 * 1024

func Add(store KVStore, node Node, h hash.Hash) []byte {
	// 将分片写入KVStore，并返回Merkle Root
	if node.Type() == FILE {
		file := node.(File)
		fileSlice := storeFile(file, store, h)
		jsonData, _ := json.Marshal(fileSlice)
		h.Write(jsonData)
		return h.Sum(nil)
	} else {
		dir := node.(Dir)
		dirSlice := storeDir(dir, store, h)
		// 序列化数据为Json字符串，并返回一个分片字符切片
		jsonData, _ := json.Marshal(dirSlice)
		// 用于写入哈希对象，便于h.Sum()计算
		h.Write(jsonData)
		// h.Sum(nil) 返回的是 h 对象当前状态下的哈希值的字节切片表示
		return h.Sum(nil)
	}
}

/*
* index：当前切割到的位置
* hight：树的层数，文件夹的层数
* returns: 对象类型和数据长度(size)
 */
func storeList(hight int, node File, store KVStore, index int, h hash.Hash) (*Object, int) {
	if hight == 1 {
		if (len(node.Bytes()) - index) <= SIZE {
			data := node.Bytes()[index:]
			blob := Object{
				Links: nil,
				Data:  data,
			}
			jsonData, _ := json.Marshal(blob)
			h.Reset()
			h.Write(jsonData)
			exists, _ := store.Has(h.Sum(nil))
			if !exists {
				store.Put(h.Sum(nil), data)
			}
			return &blob, len(data)
		}
		links := &Object{}
		totalLen := 0
		for i := 1; i <= 4096; i++ {
			end := index + SIZE
			if len(node.Bytes()) < end {
				end = len(node.Bytes())
			}
			data := node.Bytes()[index:end]
			blob := Object{
				Links: nil,
				Data:  data,
			}
			totalLen += len(data)
			jsonData, _ := json.Marshal(blob)
			h.Reset()
			h.Write(jsonData)
			exists, _ := store.Has(h.Sum(nil))
			if !exists {
				store.Put(h.Sum(nil), data)
			}
			links.Links = append(links.Links, Link{
				Hash: h.Sum(nil),
				Size: len(data),
			})
			links.Data = append(links.Data, []byte("data")...)
			index += 256 * 1024
			if index >= len(node.Bytes()) {
				break
			}
		}
		jsonData, _ := json.Marshal(links)
		h.Reset()
		h.Write(jsonData)
		exists, _ := store.Has(h.Sum(nil))
		if !exists {
			store.Put(h.Sum(nil), jsonData)
		}
		return links, totalLen
	} else {
		links := &Object{}
		totalLen := 0
		for i := 1; i <= 4096; i++ {
			if index >= len(node.Bytes()) {
				break
			}
			child, childLen := storeList(hight-1, node, store, index, h)
			totalLen += childLen
			jsonData, _ := json.Marshal(child)
			h.Reset()
			h.Write(jsonData)
			links.Links = append(links.Links, Link{
				Hash: h.Sum(nil),
				Size: childLen,
			})
			typeName := "link"
			if child.Links == nil {
				typeName = "data"
			}
			links.Data = append(links.Data, []byte(typeName)...)
		}
		jsonData, _ := json.Marshal(links)
		h.Reset()
		h.Write(jsonData)
		exists, _ := store.Has(h.Sum(nil))
		if !exists {
			store.Put(h.Sum(nil), jsonData)
		}
		return links, totalLen
	}
}

func storeFile(node File, store KVStore, h hash.Hash) *Object {
	// 如果file的size小于blob的大小256KB,则直接将数据放入blob里，类型即为blob
	if len(node.Bytes()) <= SIZE {
		data := node.Bytes()
		blob := Object{
			Links: nil,
			Data:  data,
		}
		jsonData, _ := json.Marshal(blob)
		h.Reset()
		h.Write(jsonData)
		exists, _ := store.Has(h.Sum(nil))
		if !exists {
			store.Put(h.Sum(nil), data)
		}
		return &blob
	}
	// 如果file的size大于blob的大小256KB,则将数据分片，类型为list，并存储在KVStore中
	linkLen := (len(node.Bytes()) + (SIZE - 1)) / (SIZE)
	hight := 0
	tmp := linkLen
	for {
		hight++
		// 4096是用来分片的，可以根据性能设置
		tmp /= 4096
		if tmp == 0 {
			break
		}
	}
	res, _ := storeList(hight, node, store, 0, h)
	return res
}

func storeDir(node Dir, store KVStore, h hash.Hash) *Object {
	iter := node.It()
	tree := &Object{}
	for iter.Next() {
		elem := iter.Node()
		if elem.Type() == FILE {
			file := elem.(File)
			fileSlice := storeFile(file, store, h)
			jsonData, _ := json.Marshal(fileSlice)
			h.Reset()
			h.Write(jsonData)
			tree.Links = append(tree.Links, Link{
				Hash: h.Sum(nil),
				Size: int(file.Size()),
				Name: file.Name(),
			})
			elemType := "link"
			if fileSlice.Links == nil {
				elemType = "data"
			}
			tree.Data = append(tree.Data, []byte(elemType)...)
		} else {
			dir := elem.(Dir)
			dirSlice := storeDir(dir, store, h)
			jsonData, _ := json.Marshal(dirSlice)
			h.Reset()
			h.Write(jsonData)
			tree.Links = append(tree.Links, Link{
				Hash: h.Sum(nil),
				Size: int(dir.Size()),
				Name: dir.Name(),
			})
			elemType := "tree"
			tree.Data = append(tree.Data, []byte(elemType)...)
		}
	}
	jsonData, _ := json.Marshal(tree)
	h.Reset()
	h.Write(jsonData)
	exists, _ := store.Has(h.Sum(nil))
	if !exists {
		store.Put(h.Sum(nil), jsonData)
	}
	return tree
}
