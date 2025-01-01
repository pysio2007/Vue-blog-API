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
import mongoose from 'mongoose';
import multer from 'multer';
import crypto from 'crypto';
import sharp from 'sharp';

dotenv.config();

// MongoDB 连接
mongoose.connect(process.env.MONGODB_URI || 'mongodb://localhost:27017/image-store')
  .then(() => console.log('MongoDB connected'))
  .catch(err => console.error('MongoDB connection error:', err));

// 图片 Schema
const imageSchema = new mongoose.Schema({
  hash: { type: String, required: true, unique: true },
  data: { type: Buffer, required: true },
  contentType: { type: String, required: true },
  createdAt: { type: Date, default: Date.now }
});

const Image = mongoose.model('Image', imageSchema);

// 添加计数 Schema（在现有的 Image Schema 后面）
const countSchema = new mongoose.Schema({
  key: { type: String, required: true, unique: true },
  count: { type: Number, default: 0 },
  lastUpdated: { type: Date, default: Date.now }
});

const Count = mongoose.model('Count', countSchema);

const app = express();
let lastHeartbeat: number | null = null;

const TOKEN = process.env.TOKEN;
const API_KEY = process.env.STEAM_API_KEY;
const STEAM_ID = process.env.STEAM_ID;
const IPINFO_TOKEN = process.env.IPINFO_TOKEN;
const ADMIN_TOKEN = process.env.ADMIN_TOKEN;

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

// 替换现有的 CORS 中间件
app.use(function corsMiddleware(req: Request, res: Response, next: NextFunction) {
    res.header('Access-Control-Allow-Origin', '*');
    res.header('Access-Control-Allow-Methods', 'GET, POST, PUT, DELETE, OPTIONS');
    res.header('Access-Control-Allow-Headers', 'Origin, X-Requested-With, Content-Type, Accept, Authorization');
    res.header('Access-Control-Max-Age', '86400'); // 24小时缓存预检请求结果

    // 处理 OPTIONS 预检请求
    if (req.method === 'OPTIONS') {
        res.status(200).end();
        return;
    }

    next();
});

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

// 验证管理员token的中间件
const verifyAdminToken = (req: Request, res: Response, next: NextFunction): void => {
  const authHeader = req.headers.authorization;
  if (!authHeader || authHeader !== `Bearer ${ADMIN_TOKEN}`) {
    res.status(401).json({ error: 'Unauthorized' });
    return;
  }
  next();
};

// 配置文件上传
const storage = multer.memoryStorage();
const upload = multer({ storage: storage });

// 更新调用次数的辅助函数（在其他函数定义处添加）
async function incrementCount(key: string): Promise<number> {
  const result = await Count.findOneAndUpdate(
    { key },
    { $inc: { count: 1 }, lastUpdated: new Date() },
    { upsert: true, new: true }
  );
  return result.count;
}

// 图片转换为webp的辅助函数
async function convertToWebp(buffer: Buffer): Promise<Buffer> {
  return await sharp(buffer)
    .webp({ quality: 80 })
    .toBuffer();
}

// 添加API请求计数中间件
const countApiCalls = async (req: Request, res: Response, next: NextFunction): Promise<void> => {
  const path = req.path.replace(/\/+$/, '');  // 移除尾部斜杠
  try {
    await incrementCount(path);
  } catch (error) {
    logger.error(`API count error for ${path}: ${(error as Error).message}`);
  }
  next();
};

// 应用计数中间件到所有路由
app.use(countApiCalls);

// 修改随机图片接口
app.get('/random_image', async (req: Request, res: Response): Promise<void> => {
  try {
    const count = await Image.countDocuments();
    if (count === 0) {
      res.status(404).json({ error: 'No images available' });
      return;
    }

    const random = Math.floor(Math.random() * count);
    const image = await Image.findOne().skip(random);
    
    if (!image) {
      res.status(404).json({ error: 'Image not found' });
      return;
    }

    // 增加调用次数
    await incrementCount('random_image');

    // 返回文件名为hash的webp图片
    res.set({
      'Content-Type': 'image/webp',
      'Content-Disposition': `inline; filename="${image.hash}.webp"`
    });
    res.send(image.data);
  } catch (error) {
    logger.error(`random_image error: ${(error as Error).message}`);
    res.status(500).json({ status: 'error', message: (error as Error).message });
  }
});

// 添加获取调用次数的接口
app.get('/api_stats', async (req: Request, res: Response): Promise<void> => {
  try {
    const stats = await Count.find().select('-_id key count lastUpdated');
    res.json(stats);
  } catch (error) {
    logger.error(`api_stats error: ${(error as Error).message}`);
    res.status(500).json({ status: 'error', message: (error as Error).message });
  }
});

// 获取特定接口的调用次数
app.get('/api_stats/:key', async (req: Request, res: Response): Promise<void> => {
  try {
    const { key } = req.params;
    const stat = await Count.findOne({ key }).select('-_id key count lastUpdated');
    
    if (!stat) {
      res.json({ key, count: 0, lastUpdated: null });
      return;
    }

    res.json(stat);
  } catch (error) {
    logger.error(`api_stats key error: ${(error as Error).message}`);
    res.status(500).json({ status: 'error', message: (error as Error).message });
  }
});

// 获取图片总数
app.get('/images/count', async (req: Request, res: Response): Promise<void> => {
  try {
    const count = await Image.countDocuments();
    res.json({ count });
  } catch (error) {
    logger.error(`image count error: ${(error as Error).message}`);
    res.status(500).json({ status: 'error', message: (error as Error).message });
  }
});

// 获取图片列表
app.get('/images/list', async (req: Request, res: Response): Promise<void> => {
  try {
    const page = parseInt(req.query.page as string) || 1;
    const limit = parseInt(req.query.limit as string) || 10;
    const skip = (page - 1) * limit;

    const images = await Image.find()
      .sort({ createdAt: -1 })
      .skip(skip)
      .limit(limit)
      .select('hash contentType createdAt');

    const total = await Image.countDocuments();

    res.json({
      images,
      pagination: {
        current: page,
        size: limit,
        total
      }
    });
  } catch (error) {
    logger.error(`image list error: ${(error as Error).message}`);
    res.status(500).json({ status: 'error', message: (error as Error).message });
  }
});

// 添加新图片
app.post('/images/add', verifyAdminToken, upload.single('image'), async (req: Request, res: Response): Promise<void> => {
  try {
    if (!req.file) {
      res.status(400).json({ error: 'Image file is required' });
      return;
    }

    const fileBuffer = req.file.buffer;
    const originalHash = crypto.createHash('md5').update(fileBuffer).digest('hex');

    // 转换为webp
    const webpBuffer = await convertToWebp(fileBuffer);
    const webpHash = crypto.createHash('md5').update(webpBuffer).digest('hex');

    // 检查是否已存在（基于原始文件和webp都检查）
    const exists = await Image.findOne({
      $or: [
        { hash: originalHash },
        { hash: webpHash }
      ]
    });

    if (exists) {
      res.status(409).json({ 
        error: 'Image already exists',
        existingHash: exists.hash 
      });
      return;
    }

    const image = new Image({
      hash: webpHash,
      data: webpBuffer,
      contentType: 'image/webp'
    });

    await image.save();
    
    res.status(201).json({ 
      hash: webpHash,
      contentType: 'image/webp',
      size: webpBuffer.length
    });
  } catch (error) {
    logger.error(`image add error: ${(error as Error).message}`);
    res.status(500).json({ status: 'error', message: (error as Error).message });
  }
});

// 删除图片
app.delete('/images/:hash', verifyAdminToken, async (req: Request, res: Response): Promise<void> => {
  try {
    const { hash } = req.params;
    const result = await Image.findOneAndDelete({ hash });

    if (!result) {
      res.status(404).json({ error: 'Image not found' });
      return;
    }

    res.json({ message: 'Image deleted successfully', hash });
  } catch (error) {
    logger.error(`image delete error: ${(error as Error).message}`);
    res.status(500).json({ status: 'error', message: (error as Error).message });
  }
});

// 修改特定图片获取接口
app.get('/images/:hash', async (req: Request, res: Response): Promise<void> => {
  try {
    const { hash } = req.params;
    const image = await Image.findOne({ hash });

    if (!image) {
      res.status(404).json({ error: 'Image not found' });
      return;
    }

    res.set({
      'Content-Type': 'image/webp',
      'Content-Disposition': `inline; filename="${hash}.webp"`
    });
    res.send(image.data);
  } catch (error) {
    logger.error(`get image error: ${(error as Error).message}`);
    res.status(500).json({ status: 'error', message: (error as Error).message });
  }
});

// 修改通过hash直接展示图片的接口
app.get('/i/:hash', async (req: Request, res: Response): Promise<void> => {
  try {
    const { hash } = req.params;
    const image = await Image.findOne({ hash });

    if (!image) {
      res.status(404).json({ error: 'Image not found' });
      return;
    }

    // 设置强缓存和CDN相关的响应头
    res.set({
      'Content-Type': 'image/webp',
      'Content-Disposition': `inline; filename="${hash}.webp"`,
      'Cache-Control': 'public, max-age=31536000, immutable', // 一年缓存，添加immutable
      'ETag': `"${hash}"`,
      'CDN-Cache-Control': 'max-age=31536000', // 专门针对CDN的缓存控制
      'Surrogate-Control': 'max-age=31536000', // 用于CDN缓存控制
      'Access-Control-Allow-Origin': '*', // 允许跨域访问
      'Vary': 'Accept' // 基于Accept头的内容协商
    });
    
    // 检查浏览器缓存
    const ifNoneMatch = req.header('If-None-Match');
    if (ifNoneMatch === `"${hash}"`) {
      res.status(304).send();
      return;
    }

    res.send(image.data);
  } catch (error) {
    logger.error(`get image by hash error: ${(error as Error).message}`);
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