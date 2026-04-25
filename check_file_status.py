import sqlite3
conn = sqlite3.connect('data/cleaning.db')
c = conn.cursor()

file_md5 = '6f32ed28546fb02ec39873abace37cce'

c.execute("SELECT current_step, progress, status FROM files WHERE md5=?", (file_md5,))
row = c.fetchone()
if row:
    print(f'当前步骤: {row[0]}')
    print(f'进度: {row[1]}')
    print(f'状态: {row[2]}')

c.execute("SELECT COUNT(*) FROM processing_logs WHERE file_md5=?", (file_md5,))
print(f'\n处理日志数量: {c.fetchone()[0]}')

c.execute("SELECT step, action, status FROM processing_logs WHERE file_md5=? ORDER BY id DESC LIMIT 5", (file_md5,))
print('最近的处理日志:')
for row in c.fetchall():
    print(f'  {row[0]}: {row[1]} - {row[2]}')

conn.close()
