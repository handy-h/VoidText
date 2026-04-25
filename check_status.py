import sqlite3
conn = sqlite3.connect('data/cleaning.db')
c = conn.cursor()

c.execute("SELECT md5, file_name, current_step, progress, status FROM files WHERE md5='6f32ed28546fb02ec39873abace37cce'")
row = c.fetchone()
if row:
    print(f'文件: {row[1]}')
    print(f'当前步骤: {row[2]}')
    print(f'进度: {row[3]}')
    print(f'状态: {row[4]}')
else:
    print('文件不存在')

c.execute("SELECT COUNT(*) FROM chunk_repair_cache")
print(f'\nChunk 缓存数量: {c.fetchone()[0]}')

conn.close()
