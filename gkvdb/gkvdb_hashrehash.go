package gkvdb

import (
    "g/encoding/gbinary"
    "fmt"
)

// 对数据库对应元数据列表进行重复分区
func (db *DB) checkRehash(record *Record) error {
    if record.meta.size < gMAX_META_LIST_SIZE {
        return nil
    }
    fmt.Println("need rehash for:", record)
    // 将旧空间添加进碎片管理
    db.addMtFileSpace(int(record.meta.start), record.meta.cap)

    m := make(map[int][]byte)
    p := gDEFAULT_PART_SIZE + record.index.deep + 1
    // 遍历元数据列表
    for i := 0; i < record.meta.size; i += 17 {
        buffer := record.meta.buffer[i : i + 17]
        bits   := gbinary.DecodeBytesToBits(buffer)
        hash64 := gbinary.DecodeBits(bits[0 : 64])
        part   := int(hash64%uint(p))
        if _, ok := m[part]; !ok {
            m[part] = make([]byte, 0)
        }
        m[part] = append(m[part], buffer...)
    }
    // 计算元数据大小以便分配空间
    mtsize := 0
    for _, v := range m {
        mtsize += db.getMetaCapBySize(len(v))
    }
    // 生成写入的索引数据及元数据
    mtstart  := db.getMtFileSpace(mtsize)
    mtbuffer := make([]byte, 0)
    ixbuffer := make([]byte, 0)
    for i := 0; i < p; i ++ {
        part := i
        if v, ok := m[part]; ok {
            bits    := make([]gbinary.Bit, 0)
            bits     = gbinary.EncodeBits(bits, uint(mtstart)/gMETA_BUCKET_SIZE,   36)
            bits     = gbinary.EncodeBits(bits, uint(len(v))/17,                   19)
            bits     = gbinary.EncodeBits(bits, 0,                                  1)
            mtcap   := db.getMetaCapBySize(len(v))
            mtstart += int64(mtcap)
            ixbuffer = append(ixbuffer, gbinary.EncodeBitsToBytes(bits)...)
            mtbuffer = append(mtbuffer, v...)
            for j := 0; j < int(mtcap) - len(v); j++ {
                mtbuffer = append(mtbuffer, byte(0))
            }
        } else {
            ixbuffer = append(ixbuffer, make([]byte, 7)...)
        }
    }

    // 写入重新分区后的元数据信息
    mtpf, err := db.mtfp.File()
    if err != nil {
        return err
    }
    defer mtpf.Close()
    if _, err = mtpf.File().WriteAt(mtbuffer, mtstart); err != nil {
        return err
    }

    // 写入重新分区后的索引信息
    ixpf, err := db.ixfp.File()
    if err != nil {
        return err
    }
    defer ixpf.Close()
    ixstart, err := ixpf.File().Seek(0, 2)
    if err != nil {
        return err
    }
    ixpf.File().WriteAt(ixbuffer, ixstart)
    // 修改老的索引信息
    bits    := make([]gbinary.Bit, 0)
    bits     = gbinary.EncodeBits(bits, uint(ixstart), 36)
    bits     = gbinary.EncodeBits(bits, 0,             19)
    bits     = gbinary.EncodeBits(bits, 1,              1)
    if _, err = ixpf.File().WriteAt(gbinary.EncodeBitsToBytes(bits), record.index.start); err != nil {
        return err
    }
    return nil
}