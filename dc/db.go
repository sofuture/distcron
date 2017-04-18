package dc

import (
	"encoding/json"
	"fmt"
	"github.com/docker/libkv"
	"github.com/docker/libkv/store"
	"github.com/docker/libkv/store/etcd"
	"github.com/golang/glog"
)

/*
 * KV database stores the following:
 *
 * 1. cluster leader node name
 * 2. cluster node RPC addresses - as a sole convenience for testing within a single host
 *
 * Jobs are not persisted in this particular database
 * as etcd doesn't support ever growing databases
 * https://coreos.com/etcd/docs/latest/dev-guide/limit.html
 *
 */

const CLeadershipKey = "distcron/leader"
const CRPCPrefix = "distcron/rpc"

type DB struct {
	store store.Store
}

func init() {
	etcd.Register()
}

func ConnectDB(hosts []string) (db *DB, err error) {
	db = &DB{}
	db.store, err = libkv.NewStore(store.ETCD, hosts,
		&store.Config{Bucket: "distcron"})
	return
}

func (db *DB) Put(key string, obj interface{}) error {
	data, err := json.Marshal(obj)
	if err != nil {
		ex := fmt.Errorf("Cant serialize value for key %s : %v", key, err)
		glog.Error(ex)
		return ex
	} else {
		glog.V(cLogDebug).Infof("DB %s <-- %s", key, string(data))
	}
	if err = db.store.Put(key, data, nil); err != nil {
		glog.Errorf("Can't store value for key %s : %v", key, err)
		return err
	}
	return nil
}

func (db *DB) Get(key string, obj interface{}) error {
	kv, err := db.store.Get(key)
	if err != nil {
		ex := fmt.Errorf("Cant fetch key %s: %v", key, err)
		glog.Error(ex)
		return ex
	} else {
		glog.V(cLogDebug).Infof("DB %s --> %s", key, string(kv.Value))
	}
	if err = json.Unmarshal(kv.Value, obj); err != nil {
		glog.Error(err)
		return err
	}
	return nil
}
