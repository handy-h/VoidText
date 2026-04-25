import sqlite3
conn = sqlite3.connect('data/cleaning.db')
c = conn.cursor()

c.execute("SELECT COUNT(*) FROM chunk_repair_cache WHERE source='local'")
local_count = c.fetchone()[0]

c.execute("SELECT COUNT(*) FROM chunk_repair_cache WHERE source='remote'")
remote_count = c.fetchone()[0]

c.execute("SELECT COUNT(*) FROM chunk_repair_cache")
total_count = c.fetchone()[0]

print(f'本地模型处理: {local_count} chunks')
print(f'远程API处理: {remote_count} chunks')
print(f'总计: {total_count} chunks')

conn.close()
