# 警告！ 大部分代码由AI直接生成 可读性不做保证 （懒了）

import os
from flask import Flask, jsonify, request, make_response
from dotenv import load_dotenv
import subprocess
import re
import time
from steam_web_api import Steam
import json
import requests

app = Flask(__name__)

last_heartbeat = None

load_dotenv()

TOKEN = os.getenv("TOKEN")

API_KEY = os.getenv("STEAM_API_KEY")
STEAM_ID = os.getenv("STEAM_ID")

steam = Steam(API_KEY)

def parse_ansi_colors(text):
    # Convert ANSI color codes to HTML format
    ansi_escape = re.compile(r'\x1B(?:[@-Z\\-_]|\[[0-?]*[ -/]*[@-~])')
    color_map = {
        '30': 'black', '31': 'red', '32': 'green', '33': 'yellow',
        '34': 'blue', '35': 'magenta', '36': 'cyan', '37': 'white',
        '90': 'bright-black', '91': 'bright-red', '92': 'bright-green',
        '93': 'bright-yellow', '94': 'bright-blue', '95': 'bright-magenta',
        '96': 'bright-cyan', '97': 'bright-white'
    }
    
    result = []
    current_color = None
    
    for part in ansi_escape.split(text):
        if part.startswith('\x1B'):
            color_code = part[2:-1]
            if color_code in color_map:
                current_color = color_map[color_code]
        else:
            if current_color:
                result.append(f'<span style="color:{current_color}">{part}</span>')
            else:
                result.append(part)
    
    return ''.join(result)

@app.route("/", methods=["GET"])
def hello():
    return ("你来这里干啥 喵?")

@app.after_request
def after_request(response):
    # Set CORS headers for all responses
    response.headers.add('Access-Control-Allow-Origin', '*')
    response.headers.add('Access-Control-Allow-Headers', 'Content-Type,Authorization')
    response.headers.add('Access-Control-Allow-Methods', 'GET,PUT,POST')
    return response

@app.route('/fastfetch')
def get_fastfetch():
    try:
        # Set TERM environment variable to xterm-256color
        env = os.environ.copy()
        env['TERM'] = 'xterm-256color'
        
        # Execute the fastfetch command with modified environment and capture the output
        result = subprocess.run('fastfetch -c all --logo none ', shell=True, capture_output=True, text=True, env=env)
        
        # Debug: Print raw output to console
        print("Raw output:", result.stdout)
        
        # Parse ANSI color codes
        colored_output = parse_ansi_colors(result.stdout)
        
        return jsonify({
            'status': 'success',
            'output': colored_output
        })
    except Exception as e:
        return jsonify({
            'status': 'error',
            'message': str(e)
        }), 500

@app.route("/heartbeat", methods=["POST"])
def heartbeat():
    # Token 鉴权
    if request.headers.get("Authorization") != f"Bearer {TOKEN}":
        return jsonify({"error": "Invalid token"}), 401

    # 记录心跳包时间
    global last_heartbeat
    last_heartbeat = int(time.time())
    return jsonify({"message": "Heartbeat received"})

@app.route("/check", methods=["GET"])
def check():
    # 检查心跳包时间
    if last_heartbeat is not None:
        time_diff = int(time.time()) - last_heartbeat
        return jsonify({"alive": time_diff <= 600, "last_heartbeat": last_heartbeat})
    return jsonify({"alive": False, "last_heartbeat": None})

@app.route('/steam_status')
def get_steam_status():
    user_details = steam.users.get_user_details(STEAM_ID)
    player = user_details.get("player", {})
    
    if player.get("gameextrainfo"):
        game_name = player["gameextrainfo"]
        game_id = player["gameid"]
        
        # 获取游戏详情，指定语言为简体中文，地区为中国
        game_details_url = f"http://store.steampowered.com/api/appdetails?appids={game_id}&l=schinese&cc=CN"
        response = requests.get(game_details_url)
        game_data = response.json().get(str(game_id), {}).get("data", {})
        
        short_description = game_data.get("short_description", "无可用描述")
        
        # 获取价格信息
        price_overview = game_data.get("price_overview", {})
        if price_overview:
            initial_price = price_overview.get("initial", 0)
            final_price = price_overview.get("final", 0)
            discount_percent = price_overview.get("discount_percent", 0)
            
            if final_price == 0:
                price_info = "免费"
            else:
                price_info = f"¥{final_price / 100:.2f}"
                if discount_percent > 0:
                    price_info += f" (原价 ¥{initial_price / 100:.2f}, 优惠 {discount_percent}%)"
        else:
            price_info = "免费"
        
        status = {
            "status": "在游戏中",
            "game": game_name,
            "game_id": game_id,
            "description": short_description,
            "price": price_info
        }
    else:
        status = {
            "status": "在线" if player.get("personastate") == 1 else "离线"
        }
    
    return jsonify(status)

if __name__ == '__main__':
    app.run(host='0.0.0.0', port=5000)