import sqlite3
conn = sqlite3.connect('data/cleaning.db')
c = conn.cursor()

file_md5 = '6f32ed28546fb02ec39873abace37cce'

c.execute("SELECT step, action, status, details FROM processing_logs WHERE file_md5=? ORDER BY id DESC LIMIT 15", (file_md5,))
print('处理日志:')
for row in c.fetchall():
    detail = row[3][:60] if row[3] else ''
    print(f'  {row[0]}: {row[1]} - {row[2]} - {detail}')

c.execute("SELECT COUNT(*) FROM chunk_repair_cache WHERE file_md5=?", (file_md5,))
print(f'\nChunk 缓存数量: {c.fetchone()[0]}')

c.execute("SELECT source, COUNT(*), AVG(confidence) FROM chunk_repair_cache WHERE file_md5=? GROUP BY source", (file_md5,))
print('Chunk 处理统计:')
for row in c.fetchall():
    print(f'  {row[0]}: {row[1]} chunks, 平均置信度: {row[2]:.3f}')

conn.close()
