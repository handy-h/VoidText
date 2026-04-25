import sqlite3
conn = sqlite3.connect('data/cleaning.db')
c = conn.cursor()

file_md5 = '6f32ed28546fb02ec39873abace37cce'

c.execute("SELECT chunk_id, source, confidence, processing_time_ms, length(repaired_text) FROM chunk_repair_cache WHERE file_md5=? ORDER BY chunk_id", (file_md5,))
print('Chunk 处理详情:')
print(f'{"ID":<5} {"Source":<10} {"Confidence":<12} {"Time(ms)":<10} {"Length":<8}')
print('-' * 50)
for row in c.fetchall():
    print(f'{row[0]:<5} {row[1]:<10} {row[2]:<12.4f} {row[3]:<10} {row[4]:<8}')

c.execute("SELECT source, COUNT(*), AVG(confidence), MIN(confidence), MAX(confidence) FROM chunk_repair_cache WHERE file_md5=? GROUP BY source", (file_md5,))
print('\n统计:')
print(f'{"Source":<10} {"Count":<8} {"Avg":<10} {"Min":<10} {"Max":<10}')
print('-' * 50)
for row in c.fetchall():
    print(f'{row[0]:<10} {row[1]:<8} {row[2]:<10.4f} {row[3]:<10.4f} {row[4]:<10.4f}')

conn.close()
