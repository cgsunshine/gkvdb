package gkvdb

import (
    "gitee.com/johng/gf/g/util/gtime"
    "sync"
)

// 事务操作对象
type Transaction struct {
    mu      sync.RWMutex          // 并发互斥锁
    db      *DB                   // 所属数据库
    id      int64                 // 事务编号
    start   int64                 // BinLog文件开始位置
    datamap map[string][]byte     // 事务内部的KV映射表，便于事务查询
    binlogs []*BinLog             // BinLog数据，保证写入顺序
}

// 创建一个事务
func (db *DB) Begin() *Transaction {
    tx := &Transaction {
        db      : db,
        id      : db.txid(),
        start   : -1,
        datamap : make(map[string][]byte),
        binlogs : make([]*BinLog, 0),
    }
    return tx
}

// 生成一个唯一的事务编号
func (db *DB) txid() int64 {
    return gtime.Nanosecond()
}

// 添加数据
func (tx *Transaction) Set(key, value []byte) {
    tx.mu.Lock()
    defer tx.mu.Unlock()

    tx.binlogs              = append(tx.binlogs, &BinLog{key, value})
    tx.datamap[string(key)] = value
}

// 查询数据
func (tx *Transaction) Get(key []byte) []byte {
    tx.mu.RLock()
    defer tx.mu.RUnlock()

    if v, ok := tx.datamap[string(key)]; ok {
        return v
    }
    return tx.db.Get(key)
}

// 删除数据
func (tx *Transaction) Remove(key []byte) {
    tx.mu.Lock()
    defer tx.mu.Unlock()

    tx.binlogs = append(tx.binlogs, &BinLog{key, nil})
    delete(tx.datamap, string(key))
}

// 提交数据
func (tx *Transaction) Commit() error {
    tx.mu.Lock()
    defer tx.mu.Unlock()

    start, err := tx.db.addBinLog(tx.id, tx.binlogs)
    if err != nil {
        return err
    }
    tx.start = start
    return nil
}

// 回滚数据
func (tx *Transaction) Rollback() error {
    tx.mu.Lock()
    defer tx.mu.Unlock()

    tx.start   = -1
    tx.datamap = make(map[string][]byte)
    tx.binlogs = make([]*BinLog, 0)
    return nil
}