package merkledag

import (
	"encoding/json"
	"strings"
)

// blob，list，tree这几个基本结构的基准都是4字节，所以每四个字节作为基准提取信息
const STEP = 4

// Hash to file
func Hash2File(store KVStore, hash []byte, path string, hp HashPool) []byte {
	// 根据hash和path， 返curObjBinary回对应的文件, hash对应的类型是tree
	flag, _ := store.Has(hash)
	// 判断kvstore里是否拥有hash这个key对应的value，如果有则进行拼装
	if flag {
		objBinary, _ := store.Get(hash)
		obj := binary2Obj(objBinary)
		pathArr := strings.Split(path, "\\")
		cur := 1
		return getFileByDir(obj, pathArr, cur, store)
	}
	return nil
}

// 拼装tree
func getFileByDir(obj *Object, pathArr []string, cur int, store KVStore) []byte {
	if cur >= len(pathArr) {
		return nil
	}
	index := 0
	for i := range obj.Links {
		objType := string(obj.Data[index : index+STEP])
		index += STEP
		objInfo := obj.Links[i]
		if objInfo.Name != pathArr[cur] {
			continue
		}
		switch objType {
		case TREE:
			objDirBinary, _ := store.Get(objInfo.Hash)
			objDir := binary2Obj(objDirBinary)
			ans := getFileByDir(objDir, pathArr, cur+1, store)
			if ans != nil {
				return ans
			}
		case BLOB:
			ans, _ := store.Get(objInfo.Hash)
			return ans
		case LIST:
			objLinkBinary, _ := store.Get(objInfo.Hash)
			objList := binary2Obj(objLinkBinary)
			ans := getFileByList(objList, store)
			return ans
		}
	}
	return nil
}

// 将blob和list拼装为file
func getFileByList(obj *Object, store KVStore) []byte {
	ans := make([]byte, 0)
	index := 0
	for i := range obj.Links {
		curObjType := string(obj.Data[index : index+STEP])
		index += STEP
		curObjLink := obj.Links[i]
		curObjBinary, _ := store.Get(curObjLink.Hash)
		curObj := binary2Obj(curObjBinary)
		if curObjType == BLOB {
			ans = append(ans, curObjBinary...)
		} else { //List
			tmp := getFileByList(curObj, store)
			ans = append(ans, tmp...)
		}
	}
	return ans
}

// 将新的byte数组序列化拼接并返回序列化之后的结果
func binary2Obj(objBinary []byte) *Object {
	var res Object
	json.Unmarshal(objBinary, &res)
	return &res
}
