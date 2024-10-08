from flask import Flask, jsonify
import subprocess
import re

app = Flask(__name__)

def parse_ansi_colors(text):
    # 将ANSI颜色代码转换为HTML格式
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

@app.route('/fastfetch')
def get_fastfetch():
    try:
        # 执行fastfetch命令并捕获输出
        result = subprocess.run(['fastfetch'], capture_output=True, text=True)
        
        # 解析ANSI颜色代码
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

if __name__ == '__main__':
    app.run(host='0.0.0.0', port=5000)