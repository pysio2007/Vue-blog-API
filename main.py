# 警告！ 大部分代码由AI直接生成 可读性不做保证 （懒了）

import os
from flask import Flask, jsonify, request, make_response
import subprocess
import re
import time


app = Flask(__name__)

last_heartbeat = None

TOKEN = "your_token_here"

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

if __name__ == '__main__':
    app.run(host='0.0.0.0', port=5000)