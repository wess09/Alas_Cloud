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
import time
from concurrent.futures import ThreadPoolExecutor, as_completed
import threading

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

# 进度锁与全局状态
progress_lock = threading.Lock()
table_progress = {}
stop_ui_event = threading.Event()

# ================= 数据库配置 =================

# MySQL 源数据库配置
MYSQL_CONFIG = {
    'host': '106.15.105.212',
    'port': 3306,
    'user': 'root',
    'password': 'Dn6p6mCb5QPxXxXVHcpY', 
    'database': 'alas_cloud',
    'charset': 'utf8mb4',
    'cursorclass': pymysql.cursors.DictCursor,
    'auth_plugin': 'mysql_native_password' # 必须加上这个绕过 cryptography 依赖
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

def get_mysql_conn():
    return pymysql.connect(**MYSQL_CONFIG)

def get_pg_conn():
    conn = psycopg2.connect(**PG_CONFIG)
    conn.autocommit = False
    return conn

def print_progress_ui():
    """实时渲染迁移进度"""
    with progress_lock:
        sys.stdout.write("\033[H\033[J") # 清屏
        sys.stdout.write("====== 🚢 ALAS CLOUD 多线程高速迁移中 ======\n\n")
        
        all_done = True
        for table, p in table_progress.items():
            status = p['status']
            if status != "✅": all_done = False
                
            total = p['total']
            migrated = p['migrated']
            if total > 0:
                percent = (migrated / total) * 100
                sys.stdout.write(f"[{status}] {table:20} -> {migrated}/{total} ({percent:.1f}%)\n")
            else:
                sys.stdout.write(f"[{status}] {table:20} -> {p['msg']}\n")
                
        sys.stdout.flush()
        return all_done

def migrate_table(table):
    try:
        my_conn = get_mysql_conn()
        pg_conn = get_pg_conn()
        my_cur = my_conn.cursor()
        pg_cur = pg_conn.cursor()
    except Exception as e:
        with progress_lock:
            table_progress[table] = {'status': '❌', 'total': 0, 'migrated': 0, 'msg': f"连接失败: {e}"}
        return False

    try:
        # 0. 检查全表数量与表结构
        my_cur.execute(f"SHOW COLUMNS FROM `{table}`")
        columns_info = my_cur.fetchall()
        if not columns_info:
            with progress_lock:
                table_progress[table] = {'status': '⚠️', 'total': 0, 'migrated': 0, 'msg': "无法读取结构或表不存在"}
            return False

        my_cur.execute(f"SELECT COUNT(*) as cnt FROM `{table}`")
        total_count = my_cur.fetchone()['cnt']
        
        with progress_lock:
            table_progress[table] = {'status': '⏳', 'total': total_count, 'migrated': 0, 'msg': ""}
            
        if total_count == 0:
            with progress_lock:
                table_progress[table].update({'status': '✅', 'msg': "空表跳过"})
            return True

        # ---- 建表逻辑同上 ----
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
        if pks: create_stmt += f",\n  PRIMARY KEY ({', '.join(pks)})"
        create_stmt += "\n);"
        
        pg_cur.execute(create_stmt)
        pg_cur.execute(f"TRUNCATE TABLE {table} CASCADE;") # 前置清空
        pg_conn.commit()
        
        # 1. 结构准备
        columns = [c['Field'] for c in columns_info]
        cols_str = ", ".join([f'"{c}"' for c in columns])
        placeholders = ", ".join(["%s"] * len(columns))
        insert_query = f"INSERT INTO {table} ({cols_str}) VALUES ({placeholders});"
        
        # 2. 高频分批
        batch_size = 50000 # 加大每批次抓取量
        offset = 0
        total_migrated = 0
        
        while offset < total_count:
            my_cur.execute(f"SELECT * FROM `{table}` LIMIT {batch_size} OFFSET {offset}")
            rows = my_cur.fetchall()
            if not rows: break
                
            data_to_insert = [tuple(row[col] for col in columns) for row in rows]
            extras.execute_batch(pg_cur, insert_query, data_to_insert, page_size=5000)
            pg_conn.commit()
            
            total_migrated += len(rows)
            offset += batch_size
            
            with progress_lock:
                table_progress[table]['migrated'] = total_migrated
                
        # 3. 更新自增序列
        if "id" in columns:
            try:
                seq_query = f"SELECT setval('{table}_id_seq', COALESCE((SELECT MAX(id) FROM {table}), 1), true);"
                pg_cur.execute(seq_query)
                pg_conn.commit()
            except:
                pg_conn.rollback()
                
        with progress_lock:
            table_progress[table]['status'] = '✅'
            
    except Exception as e:
        pg_conn.rollback()
        with progress_lock:
            table_progress[table] = {'status': '❌', 'total': total_count, 'migrated': total_migrated, 'msg': f"报错: {e}"}
        return False
        
    finally:
        my_cur.close()
        my_conn.close()
        pg_cur.close()
        pg_conn.close()
        
    return True

def migrate_all():
    # 初始化状态
    for t in TABLES:
        table_progress[t] = {'status': '⏳', 'total': 0, 'migrated': 0, 'msg': '排队中...'}

    def ui_updater():
        while not stop_ui_event.is_set():
            all_done = print_progress_ui()
            if all_done:
                break
            for _ in range(10): # 1秒切分成10次，防止阻塞主线程退出
                if stop_ui_event.is_set(): break
                time.sleep(0.1)
            
    # 开一个普通的(非守护)线程渲染界面
    stop_ui_event.clear()
    ui_thread = threading.Thread(target=ui_updater)
    ui_thread.start()

    # 最大8线程并发
    with ThreadPoolExecutor(max_workers=8) as executor:
        futures = {executor.submit(migrate_table, table): table for table in TABLES}
        for future in as_completed(futures):
            _ = future.result()
            
    # 安全停止并回收 UI 线程
    stop_ui_event.set()
    ui_thread.join()
    
    # 强制刷最后一次 UI
    print_progress_ui()
    print("\n🎉 全部操作已结束！多线程并发跑完啦！")

if __name__ == '__main__':
    migrate_all()
