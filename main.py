from flask import Flask, jsonify, request, make_response
import subprocess
import re

app = Flask(__name__)

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

@app.after_request
def after_request(response):
    # Set CORS headers for all responses
    response.headers.add('Access-Control-Allow-Origin', '*')
    response.headers.add('Access-Control-Allow-Headers', 'Content-Type,Authorization')
    response.headers.add('Access-Control-Allow-Methods', 'GET,PUT,POST,DELETE,OPTIONS')
    return response

@app.route('/fastfetch')
def get_fastfetch():
    try:
        # Execute the fastfetch command and capture the output
        result = subprocess.run(['fastfetch'], capture_output=True, text=True)
        
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

if __name__ == '__main__':
    app.run(host='0.0.0.0', port=5000)