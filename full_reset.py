import sqlite3
import os

file_md5 = '6f32ed28546fb02ec39873abace37cce'
db_path = 'data/cleaning.db'
states_dir = 'data/states'

# 删除状态文件
state_file = os.path.join(states_dir, f'{file_md5}_state.json')
if os.path.exists(state_file):
    os.remove(state_file)
    print(f'已删除状态文件: {state_file}')

# 重置数据库
conn = sqlite3.connect(db_path)
c = conn.cursor()

# 删除所有相关记录
tables = ['processing_logs', 'chunk_repair_cache', 'review_items', 'retry_queue']
for table in tables:
    c.execute(f"DELETE FROM {table} WHERE file_md5=?", (file_md5,))
    print(f'已删除 {c.rowcount} 条 {table} 记录')

# 重置文件状态
c.execute("UPDATE files SET current_step='cleaning', progress=0, status='pending', error_msg='' WHERE md5=?", (file_md5,))
print(f'已更新 {c.rowcount} 条文件记录')

conn.commit()
conn.close()
print('\n文件状态已完全重置')
