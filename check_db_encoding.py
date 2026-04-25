import sqlite3
conn = sqlite3.connect('data/cleaning.db')
c = conn.cursor()

file_md5 = '6f32ed28546fb02ec39873abace37cce'

c.execute("SELECT chunk_id, source, confidence, length(original_text), length(repaired_text) FROM chunk_repair_cache WHERE file_md5=? LIMIT 3", (file_md5,))
print('Chunk 基本信息:')
for row in c.fetchall():
    print(f'  ID={row[0]}, source={row[1]}, conf={row[2]:.4f}, orig_len={row[3]}, rep_len={row[4]}')

# 检查数据库编码
c.execute("PRAGMA encoding")
encoding = c.fetchone()[0]
print(f'\n数据库编码: {encoding}')

conn.close()
