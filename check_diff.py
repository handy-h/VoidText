import sqlite3

conn = sqlite3.connect('data/cleaning.db')
conn.text_factory = lambda x: x.decode('utf-8', 'ignore')
c = conn.cursor()

file_md5 = '6f32ed28546fb02ec39873abace37cce'

c.execute("SELECT chunk_id, original_text, repaired_text FROM chunk_repair_cache WHERE file_md5=? LIMIT 1", (file_md5,))
row = c.fetchone()
if row:
    chunk_id, original, repaired = row
    print(f'Chunk ID: {chunk_id}')
    print(f'\n原始文本长度: {len(original)}')
    print(f'修复文本长度: {len(repaired)}')
    
    # 计算差异
    if original == repaired:
        print('\n=== 文本完全相同 ===')
    else:
        print('\n=== 文本不同 ===')
        # 找到第一个不同的位置
        for i in range(min(len(original), len(repaired))):
            if original[i] != repaired[i]:
                print(f'第一个不同位置: {i}')
                print(f'原始: ...{original[max(0,i-30):i+30]}...')
                print(f'修复: ...{repaired[max(0,i-30):i+30]}...')
                break
        
        # 计算相同字符数
        same_chars = sum(1 for a, b in zip(original, repaired) if a == b)
        print(f'\n相同字符数: {same_chars}/{max(len(original), len(repaired))}')
        print(f'相似度: {same_chars/max(len(original), len(repaired)):.4f}')
else:
    print('没有找到 chunk 记录')

conn.close()
