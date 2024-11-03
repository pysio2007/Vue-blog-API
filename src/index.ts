import express, { Request, Response, NextFunction } from 'express';
import dotenv from 'dotenv';
import { exec } from 'child_process';
import axios from 'axios';
// @ts-ignore
import morgan from 'morgan';
import winston from 'winston';

dotenv.config();

const app = express();
let lastHeartbeat: number | null = null;

const TOKEN = process.env.TOKEN;
const API_KEY = process.env.STEAM_API_KEY;
const STEAM_ID = process.env.STEAM_ID;

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

app.use(morgan('combined', { stream: { write: (message: string) => logger.info(message.trim()) } }));

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

app.use((req: Request, res: Response, next: NextFunction) => {
    res.header('Access-Control-Allow-Origin', '*');
    res.header('Access-Control-Allow-Headers', 'Content-Type,Authorization');
    res.header('Access-Control-Allow-Methods', 'GET,PUT,POST');
    next();
});

app.get('/', (req: Request, res: Response) => {
    res.send('你来这里干啥 喵?');
});

app.get('/fastfetch', async (req: Request, res: Response) => {
    try {
        exec('fastfetch -c all --logo none', { env: { ...process.env, TERM: 'xterm-256color' } }, (error, stdout) => {
            if (error) {
                logger.error(`fastfetch error: ${(error as Error).message}`);
                res.status(500).json({ status: 'error', message: (error as Error).message });
                return;
            }
            const coloredOutput = parseAnsiColors(stdout);
            res.json({ status: 'success', output: coloredOutput });
        });
    } catch (error) {
        logger.error(`fastfetch catch error: ${(error as Error).message}`);
        res.status(500).json({ status: 'error', message: (error as Error).message });
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
        } else {
            res.json({
                status: player.personastate === 1 ? '在线' : '离线'
            });
        }
    } catch (error) {
        logger.error(`steam_status error: ${(error as Error).message}`);
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