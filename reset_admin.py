import asyncio
from telemetry_db import async_session, AdminUser, hash_password
from sqlalchemy import select

async def reset_password(username="admin", new_password="admin"):
    async with async_session() as session:
        result = await session.execute(select(AdminUser).where(AdminUser.username == username))
        user = result.scalar_one_or_none()
        
        if user:
            print(f"找到用户: {username}")
            user.password_hash = hash_password(new_password)
            print(f"密码已重置为: {new_password}")
        else:
            print(f"用户 {username} 不存在，正在创建...")
            new_user = AdminUser(username=username, password_hash=hash_password(new_password))
            session.add(new_user)
            print(f"用户已创建，密码为: {new_password}")
        
        await session.commit()

if __name__ == "__main__":
    asyncio.run(reset_password())
