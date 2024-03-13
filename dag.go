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
	// TODO 将分片写入到KVStore中，并返回Merkle Root
	if node.Type() == FILE {
		file := node.(File)
		temp := storeFile(store, file, h)
		// 序列化数据为Json字符串，并返回一个分片字符切片
		jsonMarshal, _ := json.Marshal(temp)
		// 用于写入哈希对象，便于h.Sum()计算
		h.Write(jsonMarshal)
		// h.Sum(nil) 返回的是 h 对象当前状态下的哈希值的字节切片表示
		return h.Sum(nil)
	} else {
		dir := node.(Dir)
		temp := storeDir(store, dir, h)
		jsonMarshal, _ := json.Marshal(temp)
		h.Write(jsonMarshal)
		h.Sum(nil)
	}
}

// if the file is very big which need to slice
/*
* index：当前切割到的位置
* hight：树的层数，文件夹的层数
* returns: 对象类型和数据长度(size)
*/
func storeList(store KVStore, node File, h hash.Hash, index int, hight int) (*Object,int) {
	if hight == 1 {
		// 只有一个blob，则直接将blob的类型和长度返回
		if len(node.Bytes())-index <= SIZE {
			// 获取剩余数据
			data := node.Bytes()[index:]
			// 创建blob对象
			blob :{
				Links:nil,
				Data:data,
			}
			jsonMarshal,_ := json.Marshal(blob)
			h.Write(jsonMarshal)
			store.Put(h.Sum(nil),data)
			return &blob,len(data)
		}
		// 要返回的两个数据
		links := &Object{}
		lenData := 0
		for i:=1;i<=4096;i++ {
			end := index + SIZE
			// 当节点数据的位置小于结束的位置时，调整结束位置的长度
			if end > len(node.Bytes()) {
				end = len(node.Bytes())
			}
			// 将剩余的数据记录下来
			data := node.Bytes()[index:end]
			blob := Object{
				Links:nil,
				Data:data,
			}
			lenData += len(data)
			jsonMarshal,_ := json.Marshal(blob)
			h.Write(jsonMarshal)
			store.Put(h.Sum(nil),data)
			// 此时没有名字，因为list分为blob
			links.Links = append(links.Links,Link{
				Hash:h.Sum(nil),
				Size:len(data),
			})
			links.Data = append(links.Data,[]byte("blob")...)
			index += SIZE
			if index >= len(node.Bytes()) {
				break
			}
		}
		jsonMarshal,_ := json.Marshal(links)
		h.Write(jsonMarshal)
		store.Put(h.Sum(nil),jsonMarshal)
		return links,lenData
	}else{
		links := &Object{}
		lenData := 0
		for i:=1;i<=4096;i++ {
		    if index >= len(node.Bytes()) {
		        break
		    }
			temp,lens := storeList(store,node,h,index,hight-1)
			lenData += lens
			jsonMarshal,_ := json.Marshal(temp)
			h.Write(jsonMarshal)
			links.Links = append(links.Links,Link{
			  Hash:h.Sum(nil),
			  Size:lens,  
			})
			typeName := "link"
			if temp.Links == nil {
				typeName = "blob"
			}
			links.Data = append(links.Data,[]byte(typeName)...)
		}
		jsonMarshal,_ := json.Marshal(links)
		h.Write(jsonMarshal)
		store.Put(h.Sum(nil),jsonMarshal)
		return links,lenData
	}
	
}

// execute when the node type is File
func storeFile(store KVStore, node file, h hash.Hash) *Object {
	// 如果file的size小于blob的大小256KB,则直接将数据放入blob里，类型即为blob
	if len(node.Bytes()) <= SIZE {
		// data的类型是[]byte
		data := node.Bytes()
		blob := {
			Links:nil,
			Data:data,
		}
		jsonMarshal, _ := json.Marshal(blob)
		h.Write(jsonMarshal)
		// 将value的hash作为key，value为[]byte类型的数据切片
		store.Put(h.Sum(nil), data)
		return &blob
	}
	// 如果file的size大于blob的大小256KB,则将数据分片，类型为list，并存储在KVStore中
	linkLen := (len(node.Bytes()) + (256*1024 - 1)) / (256 * 1024) // 结果向上取整
	// hight的作用是判断递归次数，也就是树的高度
	hight := 0
	temp := linkLen
	for {
		hight ++
		// 4096是用来分片的，可以根据性能设置
		temp /= 4096
		if temp == 0 {
		    break
		}
	}
	res,_ := storeList(store, node, h, 0, hight)
	return res

}

// execute when the node type is Directory
func storeDir(store KVStore, node Dir, h hash.Hash) *Object {
	it := node.It()
	treeObject := &Object{}
	for it.Next() {
		if node.Type() == FILE {
			file := node.(File)
			temp := storeFile(store, file, h)
			jsonMarshal, _ := json.Marshal(temp)
			h.Write(jsonMarshal)
			treeObject.Links = append(treeObject.Links, Link{
				Hash: h.Sum(nil),
				Size: int(file.Size()),
				Name: file.Name(),
			})
			typeName := "link"
			if temp.Next() == nil {
				typeName = "blob"
			}
			// 追加到类型为typeName的到tree的结尾
			treeObject.Data = append(treeObject.Data, []byte(typeName)...)
		}else {
			dir := node.(Dir)
			temp := storeDir(store, dir, h)
			jsonMarshal, _ := json.Marshal(temp)
			h.Write(jsonMarshal)
			// 追加到类型为link的到tree的结尾
			treeObject.Links = append(treeObject.Links, Link{
			    Hash: h.Sum(nil),
				Size: int(dir.Size()),
				Name: dir.Name(),
			})
			typeName := "tree"
			treeObject.Data = append(treeObject.Data, []byte(typeName)...)
		}
	}
	jsonMarshal, _ := json.Marshal(treeObject)
	h.Write(jsonMarshal)
	store.Put(h.Sum(nil), jsonMarshal)
	return treeObject
}
