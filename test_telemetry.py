import requests
import json
import time

url = "http://localhost:8000/api/telemetry"

data1 = {
    "device_id": "test_device_abcdef1234567890aabbcc",
    "instance_id": "inst_1",
    "month": "2026-02",
    "battle_count": 50,
    "battle_rounds": 50,
    "sortie_cost": 250,
    "akashi_encounters": 5,
    "akashi_probability": 0.1,
    "average_stamina": 12.0,
    "net_stamina_gain": -190
}

data2 = {
    "device_id": "test_device_abcdef1234567890aabbcc",
    "instance_id": "inst_1",
    "month": "2026-03",
    "battle_count": 80,
    "battle_rounds": 80,
    "sortie_cost": 400,
    "akashi_encounters": 8,
    "akashi_probability": 0.1,
    "average_stamina": 15.0,
    "net_stamina_gain": -280
}

time.sleep(2) # ensure server is up
print("Sending data1...")
r1 = requests.post(url, json=data1)
print("Feb:", r1.text)

print("Sending data2...")
r2 = requests.post(url, json=data2)
print("Mar:", r2.text)

history_url = "http://localhost:8000/api/telemetry/history?device_id=test_device_abcdef12"
print("\nFetching history...", history_url)
r3 = requests.get(history_url)
print("\nHistory:")
print(json.dumps(r3.json(), indent=2))
