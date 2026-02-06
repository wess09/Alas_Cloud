"""
数据库迁移脚本：为现有公告添加 announcement_hash 字段
"""
import asyncio
import hashlib
import time
from pathlib import Path
import os

# 设置数据目录
DATA_DIR = Path(os.getenv("DATA_DIR", "."))
DB_PATH = DATA_DIR / "telemetry.db"

async def migrate():
    """迁移数据库，为现有公告添加哈希值"""
    from sqlalchemy import text
    from sqlalchemy.ext.asyncio import create_async_engine, AsyncSession
    from sqlalchemy.orm import sessionmaker
    
    DATABASE_URL = f"sqlite+aiosqlite:///{DB_PATH}"
    engine = create_async_engine(DATABASE_URL, echo=True)
    async_session = sessionmaker(engine, class_=AsyncSession, expire_on_commit=False)
    
    async with engine.begin() as conn:
        # 检查列是否存在
        result = await conn.execute(text("PRAGMA table_info(announcements)"))
        columns = [row[1] for row in result.fetchall()]
        
        if 'announcement_hash' not in columns:
            print("正在添加 announcement_hash 列...")
            await conn.execute(text("ALTER TABLE announcements ADD COLUMN announcement_hash VARCHAR(32)"))
            print("列添加成功！")
            
            # 为现有记录生成哈希值
            result = await conn.execute(text("SELECT id, title, content, url FROM announcements WHERE announcement_hash IS NULL"))
            rows = result.fetchall()
            
            for row in rows:
                id_, title, content, url = row
                data = f"{title}|{content or ''}|{url or ''}|{time.time()}"
                hash_value = hashlib.md5(data.encode()).hexdigest()
                await conn.execute(
                    text("UPDATE announcements SET announcement_hash = :hash WHERE id = :id"),
                    {"hash": hash_value, "id": id_}
                )
                print(f"已为公告 ID={id_} 生成哈希: {hash_value[:8]}...")
            
            print(f"迁移完成！已处理 {len(rows)} 条记录")
        else:
            print("announcement_hash 列已存在，无需迁移")
    
    await engine.dispose()

if __name__ == "__main__":
    asyncio.run(migrate())
