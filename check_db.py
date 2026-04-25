#!/usr/bin/env python3
import sqlite3
import sys

def check_chunk_cache():
    db_path = "data/voidtext.db"
    try:
        conn = sqlite3.connect(db_path)
        cursor = conn.cursor()
        
        # 查询最近的缓存记录
        cursor.execute("""
            SELECT id, file_md5, chunk_id, confidence, source, 
                   LENGTH(original_text) as orig_len, 
                   LENGTH(repaired_text) as rep_len,
                   original_text = repaired_text as is_same
            FROM chunk_repair_cache 
            ORDER BY id DESC 
            LIMIT 20
        """)
        
        rows = cursor.fetchall()
        
        print("=== 最近的缓存记录 ===")
        print(f"{'ID':<5} {'File MD5':<10} {'Chunk':<6} {'Confidence':<10} {'Source':<8} {'Orig Len':<8} {'Rep Len':<8} {'Same?'}")
        print("-" * 80)
        
        for row in rows:
            id_, file_md5, chunk_id, confidence, source, orig_len, rep_len, is_same = row
            print(f"{id_:<5} {file_md5[:8]:<10} {chunk_id:<6} {confidence:<10.4f} {source:<8} {orig_len:<8} {rep_len:<8} {is_same}")
        
        # 检查置信度低但文本相同的记录
        print("\n=== 置信度低但文本相同的记录 ===")
        cursor.execute("""
            SELECT id, file_md5, chunk_id, confidence, source, 
                   LENGTH(original_text) as orig_len, 
                   LENGTH(repaired_text) as rep_len,
                   original_text, repaired_text
            FROM chunk_repair_cache 
            WHERE confidence < 0.7 AND original_text = repaired_text
            ORDER BY id DESC 
            LIMIT 10
        """)
        
        rows = cursor.fetchall()
        if rows:
            print(f"{'ID':<5} {'File MD5':<10} {'Chunk':<6} {'Confidence':<10} {'Source':<8} {'Orig Len':<8} {'Rep Len':<8}")
            print("-" * 80)
            for row in rows:
                id_, file_md5, chunk_id, confidence, source, orig_len, rep_len, orig_text, rep_text = row
                print(f"{id_:<5} {file_md5[:8]:<10} {chunk_id:<6} {confidence:<10.4f} {source:<8} {orig_len:<8} {rep_len:<8}")
                # 显示前100个字符
                print(f"  原始文本前100字符: {orig_text[:100]}")
                print(f"  修复文本前100字符: {rep_text[:100]}")
                print()
        else:
            print("没有找到置信度低但文本相同的记录")
        
        # 检查置信度分布
        print("\n=== 置信度分布 ===")
        cursor.execute("""
            SELECT 
                CASE 
                    WHEN confidence = 1.0 THEN '1.0'
                    WHEN confidence >= 0.9 THEN '0.9-1.0'
                    WHEN confidence >= 0.8 THEN '0.8-0.9'
                    WHEN confidence >= 0.7 THEN '0.7-0.8'
                    WHEN confidence >= 0.5 THEN '0.5-0.7'
                    WHEN confidence >= 0.3 THEN '0.3-0.5'
                    WHEN confidence >= 0.1 THEN '0.1-0.3'
                    ELSE '0.0-0.1'
                END as confidence_range,
                COUNT(*) as count,
                AVG(confidence) as avg_confidence
            FROM chunk_repair_cache 
            GROUP BY confidence_range
            ORDER BY confidence_range DESC
        """)
        
        rows = cursor.fetchall()
        print(f"{'置信度范围':<12} {'数量':<8} {'平均置信度':<12}")
        print("-" * 40)
        for row in rows:
            range_, count, avg_conf = row
            print(f"{range_:<12} {count:<8} {avg_conf:<12.4f}")
        
        conn.close()
        
    except Exception as e:
        print(f"数据库错误: {e}")
        sys.exit(1)

if __name__ == "__main__":
    check_chunk_cache()