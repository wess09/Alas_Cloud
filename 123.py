# ======================================================================
# 针对大表 stamina_snapshots 的专属迁移脚本 (终极完美版)
# 核心特性：主键游标分页防断连(防2013报错)、PG连接池、类型预编译转换
# ======================================================================

import sys
import time
import logging
import threading
from concurrent.futures import ThreadPoolExecutor, as_completed

def check_deps():
    try:
        import pymysql
        import psycopg2
        from psycopg2 import extras, pool
    except ImportError:
        print("❌ 缺少依赖包，请先在当前环境执行：pip install pymysql psycopg2-binary")
        sys.exit(1)

check_deps()

import pymysql
import psycopg2
from psycopg2 import extras
from psycopg2.pool import ThreadedConnectionPool

# ================= 基础配置 =================

MAX_WORKERS = 1     # 既然只有一张表，1个线程即可火力全开
BATCH_SIZE = 25000  # 稍微增大每批次数量，结合主键分页效率极高

MYSQL_CONFIG = {
    'host': '106.15.105.212',
    'port': 3306,
    'user': 'root',
    'password': 'Dn6p6mCb5QPxXxXVHcpY',
    'database': 'alas_cloud',
    'charset': 'utf8mb4',
    'connect_timeout': 60, # 增加基础网络超时容忍度
}

PG_CONFIG = {
    'host': '106.15.105.212',
    'port': 5432,
    'user': 'user123',             
    'password': 'password_0721',   
    'dbname': 'alas_cloud'         
}

# ！！！重点：只保留失败的这张表 ！！！
TABLES = [
    "stamina_snapshots"
]

# ================= 日志配置 =================
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s [%(levelname)s] [%(threadName)s] %(message)s',
    datefmt='%H:%M:%S',
    handlers=[
        logging.StreamHandler(sys.stdout),
        logging.FileHandler("migration_single.log", mode='w', encoding='utf-8')
    ]
)
logger = logging.getLogger(__name__)

pg_pool = None

def init_pools():
    global pg_pool
    try:
        logger.info("初始化 PostgreSQL 连接池...")
        pg_pool = ThreadedConnectionPool(minconn=1, maxconn=MAX_WORKERS + 2, **PG_CONFIG)
    except Exception as e:
        logger.error(f"PostgreSQL 连接池初始化失败: {e}")
        sys.exit(1)

def close_pools():
    if pg_pool:
        pg_pool.closeall()

# ================= 核心迁移逻辑 =================

def migrate_table(table):
    start_time = time.time()
    mysql_conn = None
    pg_conn = None
    ss_cur = None
    
    try:
        mysql_conn = pymysql.connect(**MYSQL_CONFIG, cursorclass=pymysql.cursors.DictCursor)
        pg_conn = pg_pool.getconn()
        pg_conn.autocommit = False
        
        mysql_cur = mysql_conn.cursor()
        pg_cur = pg_conn.cursor()

        # 1. 查询 PostgreSQL 中该表的字段类型并进行预处理 (提速核心)
        pg_cur.execute(f"SELECT column_name, data_type FROM information_schema.columns WHERE table_name = '{table}'")
        pg_schema = {row[0]: row[1].lower() for row in pg_cur.fetchall()}
        
        bool_cols = {col for col, dtype in pg_schema.items() if dtype == 'boolean'}
        json_cols = {col for col, dtype in pg_schema.items() if dtype in ('json', 'jsonb')}

        # 2. 检查 MySQL 数据量
        mysql_cur.execute(f"SELECT COUNT(*) as cnt FROM `{table}`")
        total_count = mysql_cur.fetchone()['cnt']
        
        if total_count == 0:
            logger.info(f"⏭️  [{table}] 空表，跳过迁移。")
            return True

        logger.info(f"📊 [{table}] 共 {total_count} 条数据，准备清空并迁移...")

        # 3. 清空目标表
        try:
            pg_cur.execute(f"TRUNCATE TABLE {table} CASCADE;")
            pg_conn.commit()
        except Exception as e:
            pg_conn.rollback()
            logger.error(f"[{table}] 清空目标表失败: {e}")
            return False

        # 4. 获取列信息
        mysql_cur.execute(f"SELECT * FROM `{table}` LIMIT 1")
        sample_row = mysql_cur.fetchone()
        columns = list(sample_row.keys())
        cols_str = ", ".join([f'"{c}"' for c in columns])
        insert_query = f"INSERT INTO {table} ({cols_str}) VALUES %s;"
        
        has_id_col = "id" in columns
        total_migrated = 0

        # ================== 核心提取逻辑 ==================
        if has_id_col:
            # 策略 A: 基于主键的分页 (解决超大表 2013 断连的终极方案)
            last_id = 0
            while True:
                mysql_cur.execute(f"SELECT * FROM `{table}` WHERE id > {last_id} ORDER BY id ASC LIMIT {BATCH_SIZE}")
                rows = mysql_cur.fetchall()
                if not rows:
                    break
                
                last_id = rows[-1]['id'] # 更新最后一条的 ID
                
                data_to_insert = []
                for row in rows:
                    processed_row = []
                    for col in columns:
                        val = row[col]
                        if val is not None:
                            if col in bool_cols:
                                val = bool(val)
                            elif col in json_cols and isinstance(val, (dict, list)):
                                val = extras.Json(val)
                        processed_row.append(val)
                    data_to_insert.append(tuple(processed_row))

                extras.execute_values(pg_cur, insert_query, data_to_insert, page_size=BATCH_SIZE)
                pg_conn.commit()
                
                total_migrated += len(rows)
                speed = total_migrated / (time.time() - start_time)
                percent = (total_migrated / total_count) * 100
                logger.info(f"⏳ [{table}] 进度: {total_migrated}/{total_count} ({percent:.1f}%) | 速度: {speed:.0f} 行/秒")
                
        else:
            # 策略 B: 回退使用流式游标 (适用于无 id 的小关联表)
            mysql_cur.close()
            ss_cur = mysql_conn.cursor(pymysql.cursors.SSDictCursor)
            ss_cur.execute(f"SELECT * FROM `{table}`")
            
            while True:
                rows = ss_cur.fetchmany(BATCH_SIZE)
                if not rows:
                    break
                    
                data_to_insert = []
                for row in rows:
                    processed_row = []
                    for col in columns:
                        val = row[col]
                        if val is not None:
                            if col in bool_cols:
                                val = bool(val)
                            elif col in json_cols and isinstance(val, (dict, list)):
                                val = extras.Json(val)
                        processed_row.append(val)
                    data_to_insert.append(tuple(processed_row))

                extras.execute_values(pg_cur, insert_query, data_to_insert, page_size=BATCH_SIZE)
                pg_conn.commit()
                
                total_migrated += len(rows)
                speed = total_migrated / (time.time() - start_time)
                percent = (total_migrated / total_count) * 100
                logger.info(f"⏳ [{table}] [流式] 进度: {total_migrated}/{total_count} ({percent:.1f}%) | 速度: {speed:.0f} 行/秒")

        # 5. 更新 PostgreSQL 序列
        if total_migrated > 0 and has_id_col:
            try:
                seq_query = f"SELECT setval('{table}_id_seq', COALESCE((SELECT MAX(id) FROM {table}), 1), true);"
                pg_cur.execute(seq_query)
                pg_conn.commit()
            except psycopg2.errors.UndefinedTable:
                pg_conn.rollback()

        cost_time = time.time() - start_time
        logger.info(f"✅ [{table}] 迁移完成! 耗时: {cost_time:.2f} 秒。")
        return True

    except Exception as e:
        if pg_conn:
            pg_conn.rollback()
        logger.error(f"❌ [{table}] 发生异常: {e}")
        return False

    finally:
        # 极度安全的资源释放
        if ss_cur:
            try: ss_cur.close()
            except: pass
        if mysql_conn:
            try: mysql_conn.close()
            except: pass
        if pg_conn:
            try: pg_cur.close()
            except: pass
            pg_pool.putconn(pg_conn)

# ================= 主函数 =================

def main():
    print("="*70)
    print("🚢 ALAS CLOUD 单表极速数据补完工具 (防断连版)")
    print("="*70)
    
    init_pools()
    global_start = time.time()

    success_tables = []
    failed_tables = []

    with ThreadPoolExecutor(max_workers=MAX_WORKERS, thread_name_prefix="Worker") as executor:
        future_to_table = {executor.submit(migrate_table, table): table for table in TABLES}
        
        for future in as_completed(future_to_table):
            table = future_to_table[future]
            try:
                if future.result():
                    success_tables.append(table)
                else:
                    failed_tables.append(table)
            except Exception as exc:
                failed_tables.append(table)

    close_pools()
    global_cost = time.time() - global_start
    
    print("\n" + "="*70)
    print(f"🎉 单表任务结束! 总耗时: {global_cost:.2f} 秒")
    if success_tables:
        print(f"✅ 成功补完大表: {success_tables[0]}")
    if failed_tables:
        print(f"❌ 依然失败: {failed_tables[0]}")
    print("="*70)

if __name__ == '__main__':
    main()