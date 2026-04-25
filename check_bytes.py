import sqlite3

conn = sqlite3.connect('data/cleaning.db')
conn.text_factory = bytes  # 获取原始字节
c = conn.cursor()

file_md5 = '6f32ed28546fb02ec39873abace37cce'

c.execute("SELECT chunk_id, original_text, repaired_text FROM chunk_repair_cache WHERE file_md5=? LIMIT 1", (file_md5,))
row = c.fetchone()
if row:
    chunk_id, orig_bytes, rep_bytes = row
    
    # 比较字节
    if orig_bytes == rep_bytes:
        print('字节完全相同')
    else:
        print('字节不同')
        print(f'原始字节长度: {len(orig_bytes)}')
        print(f'修复字节长度: {len(rep_bytes)}')
        
        # 找到第一个不同的字节位置
        for i in range(min(len(orig_bytes), len(rep_bytes))):
            if orig_bytes[i] != rep_bytes[i]:
                print(f'第一个不同字节位置: {i}')
                print(f'原始字节: {orig_bytes[max(0,i-10):i+10]}')
                print(f'修复字节: {rep_bytes[max(0,i-10):i+10]}')
                break
else:
    print('没有找到 chunk 记录')

conn.close()
