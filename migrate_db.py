# ======================================================================
# MySQL 到 PostgreSQL 数据迁移脚本
# 运行前准备：
# 1. 启动你的后端项目，让 GORM 在 PostgreSQL 里先自动建立所有表结构。
# 2. 安装 Python 依赖: 
#    pip install pymysql psycopg2-binary
# 
# 注意：请自行核对 PG_CONFIG 里面的密码等信息是否符合 1Panel 的配置
# ======================================================================

import sys

def check_deps():
    try:
        import pymysql
        import psycopg2
        from psycopg2 import extras
    except ImportError:
        print("❌ 缺少依赖包，请先在当前环境执行：pip install pymysql psycopg2-binary")
        sys.exit(1)

check_deps()

import pymysql
import psycopg2
from psycopg2 import extras

# ================= 数据库配置 =================

# MySQL 源数据库配置
MYSQL_CONFIG = {
    'host': '106.15.105.212',
    'port': 3306,
    'user': 'root',
    'password': 'Dn6p6mCb5QPxXxXVHcpY', # 根据你原先的 docker-compose
    'database': 'alas_cloud',
    'charset': 'utf8mb4',
    'cursorclass': pymysql.cursors.DictCursor
}

# PostgreSQL 目标数据库配置
PG_CONFIG = {
    'host': '106.15.105.212',     # 如果本地运行脚本，请用外网IP。如果在容器内，可以用 1Panel-postgresql-BBVv
    'port': 5432,               # 请确保 1Panel 已开放此端口
    'user': 'user123',         # 【⚠️请修改】 1Panel 给你的 PostgreSQL 的用户名
    'password': 'password_0721',             # 【⚠️请修改】 1Panel 给你的 PostgreSQL 的密码
    'dbname': 'alas_cloud'      # 【⚠️请确认】 PostgreSQL 对应的数据库名
}

# 需要迁移的表列表 (和模型一致的蛇形命名)
TABLES = [
    "user_profiles",
    "admin_users",
    "announcements",
    "system_configs",
    "banned_users",
    "telemetry_data",
    "azurstat_reports",
    "azurstat_item_drops",
    "reports",
    "stamina_snapshots",
    "stamina_kline"
]

def migrate():
    print("====== 🚢 ALAS CLOUD 数据迁移脚本 (MySQL -> PostgreSQL) ======")
    
    # 建立 MySQL 连接
    print(f"\n[1/3] 正在连接到 MySQL [{MYSQL_CONFIG['host']}:{MYSQL_CONFIG['port']}]...")
    try:
        mysql_conn = pymysql.connect(**MYSQL_CONFIG)
    except Exception as e:
        print(f"❌ 连接 MySQL 失败，请检查配置或防火墙: {e}")
        return

    # 建立 PostgreSQL 连接
    print(f"[2/3] 正在连接到 PostgreSQL [{PG_CONFIG['host']}:{PG_CONFIG['port']}]...")
    try:
        pg_conn = psycopg2.connect(**PG_CONFIG)
        pg_conn.autocommit = False # 使用显式事务
    except Exception as e:
        print(f"❌ 连接 PostgreSQL 失败，请检查密码和端口是否开放: {e}")
        return

    mysql_cur = mysql_conn.cursor()
    pg_cur = pg_conn.cursor()
    
    print("\n[3/3] 开始迁移数据表...")
    for table in TABLES:
        # 0. 在 PG 中自动建表
        try:
            mysql_cur.execute(f"SHOW COLUMNS FROM `{table}`")
            columns_info = mysql_cur.fetchall()
            if not columns_info:
                print(f"⚠️ [跳过] 无法读取 MySQL 表 {table} 结构。")
                continue
                
            create_defs = []
            pks = []
            for col in columns_info:
                field = col['Field']
                ctype = str(col['Type']).lower()
                extra = str(col.get('Extra', '')).lower()
                key = str(col.get('Key', '')).upper()
                
                pg_type = 'TEXT'
                if 'int' in ctype:
                    if 'bigint' in ctype: pg_type = 'BIGINT'
                    elif 'tinyint' in ctype: pg_type = 'SMALLINT'
                    else: pg_type = 'INTEGER'
                elif 'datetime' in ctype or 'timestamp' in ctype:
                    pg_type = 'TIMESTAMP'
                elif 'float' in ctype or 'double' in ctype or 'decimal' in ctype:
                    pg_type = 'DOUBLE PRECISION'
                elif 'json' in ctype:
                    pg_type = 'JSONB'
                
                if 'auto_increment' in extra:
                    pg_type = 'BIGSERIAL' if pg_type == 'BIGINT' else 'SERIAL'
                
                create_defs.append(f'"{field}" {pg_type}')
                if key == 'PRI':
                    pks.append(f'"{field}"')
                    
            create_stmt = f"CREATE TABLE IF NOT EXISTS {table} (\n  " + ",\n  ".join(create_defs)
            if pks:
                create_stmt += f",\n  PRIMARY KEY ({', '.join(pks)})"
            create_stmt += "\n);"
            
            pg_cur.execute(create_stmt)
            pg_conn.commit()
            print(f"🔧 [建表] 已确保表 {table} 结构在 PostgreSQL 中存在。")
        except Exception as e:
            pg_conn.rollback()
            print(f"⚠️ [建表错误] {table}: {e}")

        # 1. 前置清空与进度准备
        try:
            # 清空目标表以防止主键冲突（级联清空以处理外键依赖）
            pg_cur.execute(f"TRUNCATE TABLE {table} CASCADE;")
            pg_conn.commit()
            
            mysql_cur.execute(f"SELECT COUNT(*) as cnt FROM `{table}`")
            total_count = mysql_cur.fetchone()['cnt']
        except Exception as e:
            print(f"⚠️ [准备失败] {table}: {e}")
            pg_conn.rollback()
            continue
            
        if total_count == 0:
            print(f"⏭️  [空表] {table} 数据为空，跳过。")
            continue
            
        print(f"⏳ 表 {table} 共有 {total_count} 条数据，开始分批迁移...")
        
        # 尝试获取表列名
        mysql_cur.execute(f"SELECT * FROM `{table}` LIMIT 1")
        sample_row = mysql_cur.fetchone()
        columns = list(sample_row.keys())
        cols_str = ", ".join([f'"{c}"' for c in columns])
        placeholders = ", ".join(["%s"] * len(columns))
        insert_query = f"INSERT INTO {table} ({cols_str}) VALUES ({placeholders});"

        # 2. 分批读取与插入
        batch_size = 20000
        offset = 0
        total_migrated = 0
        
        while offset < total_count:
            try:
                mysql_cur.execute(f"SELECT * FROM `{table}` LIMIT {batch_size} OFFSET {offset}")
                rows = mysql_cur.fetchall()
                if not rows:
                    break
                    
                data_to_insert = [tuple(row[col] for col in columns) for row in rows]
                
                # 开始 PG 插入
                extras.execute_batch(pg_cur, insert_query, data_to_insert, page_size=2000)
                pg_conn.commit()
                
                total_migrated += len(rows)
                offset += batch_size
                print(f"   [进度] {table}: {total_migrated} / {total_count} ({(total_migrated/total_count)*100:.1f}%)")
                
            except Exception as e:
                pg_conn.rollback()
                print(f"❌ [失败] 迁移表 {table} 时出错在 batch {offset}: {e}")
                break
                
        # 3. ======== 最关键的一步：更新自增序列 ======== 
        if total_migrated > 0 and "id" in columns:
            try:
                seq_query = f"SELECT setval('{table}_id_seq', COALESCE((SELECT MAX(id) FROM {table}), 1), true);"
                pg_cur.execute(seq_query)
                pg_conn.commit()
                print(f"   🔧 序列 {table}_id_seq 已同步")
            except psycopg2.errors.UndefinedTable:
                pg_conn.rollback()
                pass
                
        print(f"✅ [完成] 表 {table} 迁移结束。")

    # 清理资源
    mysql_cur.close()
    mysql_conn.close()
    pg_cur.close()
    pg_conn.close()
    print("\n🎉 全部操作已结束！请重新启动你的后端服务。")

if __name__ == '__main__':
    migrate()
