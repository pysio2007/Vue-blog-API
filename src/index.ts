import express, { Request, Response, NextFunction } from 'express';
import dotenv from 'dotenv';
import { exec } from 'child_process';
import axios from 'axios';
import { promisify } from 'util';
import fs from 'fs/promises';
import path from 'path';
// @ts-ignore
import morgan from 'morgan';
import winston from 'winston';

dotenv.config();

const app = express();
let lastHeartbeat: number | null = null;

const countFilePath = path.join(__dirname, 'random_image_count.txt');

const TOKEN = process.env.TOKEN;
const API_KEY = process.env.STEAM_API_KEY;
const STEAM_ID = process.env.STEAM_ID;
const IPINFO_TOKEN = process.env.IPINFO_TOKEN;
const GITHUB_CLIENT_ID = process.env.GITHUB_CLIENT_ID;
const GITHUB_CLIENT_SECRET = process.env.GITHUB_CLIENT_SECRET;
const ALLOWED_USERS = ['pysio2007']; 

const logger = winston.createLogger({
    level: 'info',
    format: winston.format.combine(
        winston.format.timestamp(),
        winston.format.json()
    ),
    transports: [
        new winston.transports.Console(),
        new winston.transports.File({ filename: 'combined.log' })
    ]
});

// Express中间件配置
app.use(express.json());
app.use(morgan('combined', { stream: { write: (message: string) => logger.info(message.trim()) } }));

// 文件读写函数
async function readCountFromFile(): Promise<number> {
    try {
        const data = await fs.readFile(countFilePath, 'utf8');
        return parseInt(data, 10) || 0;
    } catch (error) {
        return 0;
    }
}

async function writeCountToFile(count: number): Promise<void> {
    await fs.writeFile(countFilePath, count.toString(), 'utf8');
}

function parseAnsiColors(text: string): string {
    const ansiEscape = /\x1B(?:[@-Z\\-_]|\[[0-?]*[ -/]*[@-~])/g;
    const colorMap: { [key: string]: string } = {
        '30': 'black', '31': 'red', '32': 'green', '33': 'yellow',
        '34': 'blue', '35': 'magenta', '36': 'cyan', '37': 'white',
        '90': 'bright-black', '91': 'bright-red', '92': 'bright-green',
        '93': 'bright-yellow', '94': 'bright-blue', '95': 'bright-magenta',
        '96': 'bright-cyan', '97': 'bright-white'
    };

    let result: string[] = [];
    let currentColor: string | null = null;

    text.split(ansiEscape).forEach(part => {
        if (part.startsWith('\x1B')) {
            const colorCode = part.slice(2, -1);
            if (colorMap[colorCode]) {
                currentColor = colorMap[colorCode];
            }
        } else {
            if (currentColor) {
                result.push(`<span style="color:${currentColor}">${part}</span>`);
            } else {
                result.push(part);
            }
        }
    });

    return result.join('');
}

// CORS中间件
app.use((req: Request, res: Response, next: NextFunction) => {
    res.header('Access-Control-Allow-Origin', '*');
    res.header('Access-Control-Allow-Headers', 'Content-Type,Authorization');
    res.header('Access-Control-Allow-Methods', 'GET,PUT,POST');
    next();
});

// 原有路由
app.get('/', (req: Request, res: Response) => {
    res.send('你来这里干啥 喵?');
});

app.get('/fastfetch', async (req: Request, res: Response) => {
    try {
        const execAsync = promisify(exec);
        const { stdout } = await execAsync('fastfetch -c all --logo none', { env: { ...process.env, TERM: 'xterm-256color' } });
        const coloredOutput = parseAnsiColors(stdout);
        res.json({ status: 'success', output: coloredOutput });
    } catch (error) {
        logger.error(`fastfetch error: ${(error as Error).message}`);
        res.status(500).json({ status: 'error', message: (error as Error).message });
    }
});

// GitHub OAuth相关路由
app.post('/auth/github', async (req: Request, res: Response): Promise<void> => {
    const { code } = req.body;
    
    try {
        const tokenRes = await axios.post('https://github.com/login/oauth/access_token', {
            client_id: GITHUB_CLIENT_ID,
            client_secret: GITHUB_CLIENT_SECRET,
            code
        }, {
            headers: { Accept: 'application/json' }
        });

        const { access_token } = tokenRes.data;

        const userRes = await axios.get('https://api.github.com/user', {
            headers: { Authorization: `token ${access_token}` }
        });

        const user = userRes.data;

        if (!ALLOWED_USERS.includes(user.login)) {
            res.status(403).json({ error: 'Unauthorized user' });
            return;
        }

        res.json({ access_token });
    } catch (error) {
        res.status(500).json({ error: (error as Error).message });
    }
});

app.get('/repo/files', async (req: Request, res: Response) => {
    const token = req.headers.authorization?.split(' ')[1];
    
    try {
        const filesRes = await axios.get(
            'https://api.github.com/repos/PysioHub/Vue-blog-Dev/contents', {
            headers: { Authorization: `token ${token}` }
        });

        res.json(filesRes.data);
    } catch (error) {
        res.status(500).json({ error: (error as Error).message });
    }
});

app.put('/repo/files/:path', async (req: Request, res: Response) => {
    const token = req.headers.authorization?.split(' ')[1];
    const { content, sha, message } = req.body;
    const filePath = req.params.path;

    try {
        const updateRes = await axios.put(
            `https://api.github.com/repos/PysioHub/Vue-blog-Dev/contents/${filePath}`,
            {
                message: message || 'Update file',
                content: Buffer.from(content).toString('base64'),
                sha
            },
            {
                headers: { Authorization: `token ${token}` }
            }
        );

        res.json(updateRes.data);
    } catch (error) {
        res.status(500).json({ error: (error as Error).message });
    }
});

app.post('/heartbeat', (req: Request, res: Response) => {
    if (req.headers.authorization !== `Bearer ${TOKEN}`) {
        logger.warn('Invalid token in heartbeat');
        res.status(401).json({ error: 'Invalid token' });
        return;
    }
    lastHeartbeat = Math.floor(Date.now() / 1000);
    res.json({ message: 'Heartbeat received' });
});

app.get('/check', (req: Request, res: Response) => {
    if (lastHeartbeat !== null) {
        const timeDiff = Math.floor(Date.now() / 1000) - lastHeartbeat;
        res.json({ alive: timeDiff <= 600, last_heartbeat: lastHeartbeat });
    } else {
        res.json({ alive: false, last_heartbeat: null });
    }
});

app.get('/steam_status', async (req: Request, res: Response) => {
    try {
        const userDetailsUrl = `https://api.steampowered.com/ISteamUser/GetPlayerSummaries/v0002/?key=${API_KEY}&steamids=${STEAM_ID}`;
        const userDetailsResponse = await axios.get(userDetailsUrl);
        const player = userDetailsResponse.data.response.players[0] || {};

        if (player.gameextrainfo) {
            const gameName = player.gameextrainfo;
            const gameId = player.gameid;

            const gameDetailsUrl = `https://store.steampowered.com/api/appdetails?appids=${gameId}&l=schinese&cc=CN`;
            const gameDetailsResponse = await axios.get(gameDetailsUrl);
            const gameData = gameDetailsResponse.data[gameId].data || {};

            const shortDescription = gameData.short_description || '无可用描述';

            const priceOverview = gameData.price_overview || {};
            let priceInfo = '免费';
            if (priceOverview.final) {
                priceInfo = `¥${(priceOverview.final / 100).toFixed(2)}`;
                if (priceOverview.discount_percent > 0) {
                    priceInfo += ` (原价 ¥${(priceOverview.initial / 100).toFixed(2)}, 优惠 ${priceOverview.discount_percent}%)`;
                }
            }

            const playerGamesUrl = `https://api.steampowered.com/IPlayerService/GetOwnedGames/v0001/?key=${API_KEY}&steamid=${STEAM_ID}&format=json`;
            const playerGamesResponse = await axios.get(playerGamesUrl);
            const playerGamesData = playerGamesResponse.data.response || {};

            let totalPlaytime = 0;
            for (const game of playerGamesData.games || []) {
                if (game.appid.toString() === gameId) {
                    totalPlaytime = game.playtime_forever;
                    break;
                }
            }

            const playtimeHours = Math.floor(totalPlaytime / 60);
            const playtimeMinutes = totalPlaytime % 60;

            const achievementsUrl = `https://api.steampowered.com/ISteamUserStats/GetPlayerAchievements/v0001/?appid=${gameId}&key=${API_KEY}&steamid=${STEAM_ID}`;
            try {
                const achievementsResponse = await axios.get(achievementsUrl);
                const achievementsData = achievementsResponse.data.playerstats || {};

                const achievements = achievementsData.achievements || [];
                const totalAchievements = achievements.length;
                const completedAchievements = achievements.filter((achievement: any) => achievement.achieved === 1).length;
                const achievementPercentage = totalAchievements > 0 ? (completedAchievements / totalAchievements * 100).toFixed(2) : '0.00';

                res.json({
                    status: '在游戏中',
                    game: gameName,
                    game_id: gameId,
                    description: shortDescription,
                    price: priceInfo,
                    playtime: `${playtimeHours}小时${playtimeMinutes}分钟`,
                    achievement_percentage: `${achievementPercentage}%`
                });
            } catch (achievementError: any) {
                if (achievementError.response && achievementError.response.data && achievementError.response.data.playerstats && achievementError.response.data.playerstats.error === "Requested app has no stats") {
                    res.json({
                        status: '在游戏中',
                        game: gameName,
                        game_id: gameId,
                        description: shortDescription,
                        price: priceInfo,
                        playtime: `${playtimeHours}小时${playtimeMinutes}分钟`,
                        achievement_percentage: '无成就数据'
                    });
                } else {
                    throw achievementError;
                }
            }
        } else {
            res.json({
                status: player.personastate === 1 ? '在线' : '离线'
            });
        }
    } catch (error) {
        logger.error(`steam_status error: ${(error as Error).message}`, {
            url: (error as any).config?.url,
            data: (error as any).response?.data
        });
        res.status(500).json({ status: 'error', message: (error as Error).message });
    }
});

app.get('/ipcheck', async (req: Request, res: Response): Promise<void> => {
    const ip = req.query.ip as string;
    if (!ip) {
        res.status(400).json({ status: 'error', message: 'IP 参数是必须的' });
        return;
    }

    try {
        const ipInfoUrl = `https://ipinfo.io/widget/demo/${ip}`;
        const ipInfoResponse = await axios.get(ipInfoUrl);
        const ipInfoData = ipInfoResponse.data;

        res.json(ipInfoData);
    } catch (error) {
        if (axios.isAxiosError(error) && error.response?.status === 429) {
            try {
                const fallbackUrl = `https://ipinfo.io/${ip}?token=${IPINFO_TOKEN}`;
                const fallbackResponse = await axios.get(fallbackUrl);
                const fallbackData = fallbackResponse.data;
                fallbackData["429"] = "true";

                res.json(fallbackData);
            } catch (fallbackError) {
                logger.error(`ipcheck fallback error: ${(fallbackError as Error).message}`);
                res.status(500).json({ status: 'error', message: (fallbackError as Error).message });
            }
        } else {
            logger.error(`ipcheck error: ${(error as Error).message}`);
            res.status(500).json({ status: 'error', message: (error as Error).message });
        }
    }
});

app.get('/random_image', async (req: Request, res: Response) => {
    try {
        // 读取当前计数
        let count = await readCountFromFile();
        // 增加计数
        count += 1;
        // 写回新的计数
        await writeCountToFile(count);

        const response = await axios.get('https://randomimg.pysio.online/url.csv');
        const urls = response.data.split('\n').filter((url: string) => url.trim() !== '');
        const randomUrl = urls[Math.floor(Math.random() * urls.length)];
        res.redirect(302, randomUrl);
    } catch (error) {
        logger.error(`random_image error: ${(error as Error).message}`);
        res.status(500).json({ status: 'error', message: (error as Error).message });
    }
});

app.get('/random_image_count', async (req: Request, res: Response) => {
    try {
        const count = await readCountFromFile();
        res.json({ count });
    } catch (error) {
        logger.error(`random_image_count error: ${(error as Error).message}`);
        res.status(500).json({ status: 'error', message: (error as Error).message });
    }
});

app.get('/egg', (req: Request, res: Response) => {
    res.send('Oops!');
});

app.get('/404', (req: Request, res: Response) => {
    res.send('404 Not Found');
});

app.get('/50x', (req: Request, res: Response) => {
    res.send('Server Down');
});

app.listen(5000, () => {
    logger.info('Server is running on port 5000');
});