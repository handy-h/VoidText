import sqlite3
conn = sqlite3.connect('data/cleaning.db')
c = conn.cursor()

c.execute("UPDATE files SET current_step='pending', progress=0, status='pending' WHERE md5='6f32ed28546fb02ec39873abace37cce'")
conn.commit()
print(f'Updated {c.rowcount} rows')

c.execute("DELETE FROM chunk_repair_cache")
conn.commit()
print('Cleared chunk_repair_cache')

conn.close()
