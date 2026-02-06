"""
遥测数据库迁移脚本
将旧版数据库(只有 device_id)迁移到新版(device_id + instance_id)

使用方法:
    python migrate_telemetry_db.py

注意:
    - 会自动备份旧数据库为 telemetry.db.backup
    - 为所有现有记录添加默认 instance_id = "default"
"""

import sqlite3
import shutil
from pathlib import Path
from datetime import datetime


def migrate_database():
    """执行数据库迁移"""
    
    db_path = Path(__file__).parent / "telemetry.db"
    backup_path = Path(__file__).parent / f"telemetry.db.backup_{datetime.now().strftime('%Y%m%d_%H%M%S')}"
    
    # 检查数据库是否存在
    if not db_path.exists():
        print("❌ 未找到 telemetry.db 文件,无需迁移")
        return False
    
    print(f"📦 开始迁移数据库: {db_path}")
    
    # 1. 备份原数据库
    print(f"💾 备份原数据库到: {backup_path}")
    shutil.copy2(db_path, backup_path)
    
    # 2. 连接数据库
    conn = sqlite3.connect(db_path)
    cursor = conn.cursor()
    
    try:
        # 3. 检查是否已经有 instance_id 列
        cursor.execute("PRAGMA table_info(telemetry_data)")
        columns = [col[1] for col in cursor.fetchall()]
        
        if 'instance_id' in columns:
            print("✅ 数据库已包含 instance_id 列,无需迁移")
            conn.close()
            return True
        
        print("🔧 检测到旧版数据库结构,开始迁移...")
        
        # 4. 创建新表结构
        cursor.execute("""
            CREATE TABLE IF NOT EXISTS telemetry_data_new (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                device_id TEXT NOT NULL,
                instance_id TEXT NOT NULL,
                month TEXT NOT NULL,
                battle_count INTEGER NOT NULL,
                battle_rounds INTEGER NOT NULL,
                sortie_cost INTEGER NOT NULL,
                akashi_encounters INTEGER NOT NULL,
                akashi_probability REAL NOT NULL,
                average_stamina REAL NOT NULL,
                net_stamina_gain INTEGER NOT NULL,
                created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
                updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
                CONSTRAINT uix_device_instance UNIQUE (device_id, instance_id)
            )
        """)
        
        # 5. 迁移数据 (为所有记录添加默认 instance_id = "default")
        cursor.execute("""
            INSERT INTO telemetry_data_new 
            (id, device_id, instance_id, month, battle_count, battle_rounds, 
             sortie_cost, akashi_encounters, akashi_probability, average_stamina, 
             net_stamina_gain, created_at, updated_at)
            SELECT 
                id, device_id, 'default' as instance_id, month, battle_count, 
                battle_rounds, sortie_cost, akashi_encounters, akashi_probability, 
                average_stamina, net_stamina_gain, created_at, updated_at
            FROM telemetry_data
        """)
        
        migrated_count = cursor.rowcount
        print(f"📊 已迁移 {migrated_count} 条记录")
        
        # 6. 删除旧表,重命名新表
        cursor.execute("DROP TABLE telemetry_data")
        cursor.execute("ALTER TABLE telemetry_data_new RENAME TO telemetry_data")
        
        # 7. 创建索引
        cursor.execute("CREATE INDEX IF NOT EXISTS ix_telemetry_data_device_id ON telemetry_data (device_id)")
        cursor.execute("CREATE INDEX IF NOT EXISTS ix_telemetry_data_instance_id ON telemetry_data (instance_id)")
        cursor.execute("CREATE INDEX IF NOT EXISTS ix_telemetry_data_month ON telemetry_data (month)")
        
        # 8. 提交更改
        conn.commit()
        print("✅ 数据库迁移成功!")
        print(f"📝 所有记录的 instance_id 已设置为 'default'")
        print(f"💡 提示: 备份文件保存在 {backup_path}")
        
        return True
        
    except Exception as e:
        print(f"❌ 迁移失败: {str(e)}")
        conn.rollback()
        
        # 恢复备份
        print(f"🔄 正在从备份恢复...")
        conn.close()
        shutil.copy2(backup_path, db_path)
        print("✅ 已恢复到迁移前状态")
        
        return False
        
    finally:
        conn.close()


def verify_migration():
    """验证迁移结果"""
    db_path = Path(__file__).parent / "telemetry.db"
    
    if not db_path.exists():
        return
    
    conn = sqlite3.connect(db_path)
    cursor = conn.cursor()
    
    try:
        # 检查表结构
        cursor.execute("PRAGMA table_info(telemetry_data)")
        columns = {col[1]: col[2] for col in cursor.fetchall()}
        
        print("\n📋 当前数据库结构:")
        for col_name, col_type in columns.items():
            print(f"  - {col_name}: {col_type}")
        
        # 检查数据
        cursor.execute("SELECT COUNT(*) FROM telemetry_data")
        total_count = cursor.fetchone()[0]
        
        cursor.execute("SELECT COUNT(DISTINCT device_id) FROM telemetry_data")
        device_count = cursor.fetchone()[0]
        
        cursor.execute("SELECT COUNT(DISTINCT instance_id) FROM telemetry_data")
        instance_count = cursor.fetchone()[0]
        
        print(f"\n📊 数据统计:")
        print(f"  - 总记录数: {total_count}")
        print(f"  - 设备数: {device_count}")
        print(f"  - 实例数: {instance_count}")
        
    finally:
        conn.close()


if __name__ == "__main__":
    print("=" * 60)
    print("遥测数据库迁移工具")
    print("=" * 60)
    print()
    
    success = migrate_database()
    
    if success:
        print()
        verify_migration()
        print()
        print("=" * 60)
        print("✅ 迁移完成!")
        print("=" * 60)
    else:
        print()
        print("=" * 60)
        print("❌ 迁移失败或无需迁移")
        print("=" * 60)
