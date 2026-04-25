import sqlite3
conn = sqlite3.connect('data/cleaning.db')
c = conn.cursor()

file_md5 = '6f32ed28546fb02ec39873abace37cce'

c.execute("SELECT rules_config FROM files WHERE md5=?", (file_md5,))
row = c.fetchone()
if row:
    print(f'规则配置: {row[0]}')
else:
    print('文件不存在')

conn.close()
