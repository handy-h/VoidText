import sqlite3
conn = sqlite3.connect('data/cleaning.db')
c = conn.cursor()

# 检查审核项表
c.execute("SELECT COUNT(*) FROM review_items WHERE file_md5='6f32ed28546fb02ec39873abace37cce'")
review_count = c.fetchone()[0]
print(f'审核项数量: {review_count}')

# 检查 chunk_repair_cache 的变更
c.execute("SELECT source, COUNT(*), AVG(confidence) FROM chunk_repair_cache GROUP BY source")
print('\nChunk 处理统计:')
for row in c.fetchall():
    print(f'  {row[0]}: {row[1]} chunks, 平均置信度: {row[2]:.3f}')

# 检查最近的 chunk 记录
c.execute("SELECT chunk_id, source, confidence, length(repaired_text) FROM chunk_repair_cache ORDER BY chunk_id DESC LIMIT 5")
print('\n最近 5 个 chunk:')
for row in c.fetchall():
    print(f'  Chunk {row[0]}: source={row[1]}, confidence={row[2]:.3f}, len={row[3]}')

conn.close()
