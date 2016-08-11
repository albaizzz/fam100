---
layout: default
date: 2016-08-02
title:  "why is the bot slow sometimes"
---

```
TLDR; The bot is fast enough, but it sending too many messages and Telegram limits this.
I'll be focusing my effort on making it more efficient by rewriting the code.
For now please endure this with me. 

Also I have a daily JOB and I only do this for fun :)
so I can't really promise fast delivery
```

Before answering the question let's see the scale of what fam100 bot currently processed. **all graph shown below is CEST time, for WIB add 5 hours** 

![user and player graph]({{ site.baseurl}}/images/20160802/active_game_and_players.png)

On the left side you see that during peak there are about **500**  active game with **2000** users playing at the same time. But this is not entirely the problem. On peak this mean 2000 people are typing answer at the same time. But this is not necessary the problem. How many message does the bot processed?

<img src="{{ site.baseurl}}/images/20160802/incoming_message_perday.png" alt="incoming message per day" width="50%" style="float:left">
<img src="{{ site.baseurl}}/images/20160802/incoming_message_persec.png" alt="incoming message per day" width="50%" style="">

The left graph shows how many message processed per day in one week. It's average about **6 million** messages per day. The right graph shows how many message sent per seconds. On peak it took about **500** messages per second (that's 3000 messages per minute). But how long is it to process a single message? 

![incoming message per day]({{ site.baseurl}}/images/20160802/service_time.png){: .center-image .half }

The graph shows that on average it took about **170 micro seconds** to process a single message. This would amount to **5882** message per seconds. And at 99th percentile it took about  **2.5 ms** (this would amount to 400 message per seconds. So I believe, even with 10x the current load, the bot server will still able to process messages (assuming we can get the message fast enough from telegram).  
So it doesn't matter if you rewrite the code, at this scale you will have the same problem.

So if processing the message is not the problem, than what is? If you read the [Telegram F.A.Q.](https://core.telegram.org/bots/faq#my-bot-is-hitting-limits-how-do-i-avoid-this) you see that telegram limit message to be sent about 30 messages per seconds. Sometimes the server even returns HTTP 429 (Too many Request) and when that happened, the bot needs to wait a couple of minutes (I think it's about 2 minutes) before it able to send messages again.

You can clearly see it in the graph response time below. 

![response time]({{ site.baseurl}}/images/20160802/response_time.png){: .center-image }

The orange (left axis) represent 95th percentile of how long it takes to send a single message, It's stable around 250ms. While the red one (right axis), at peak time a lot of request is took more than 5 seconds (timeout). This is when the bot failed sending message

The current solution for this problem is by dropping unimportant message such as

![joint confirmation]({{ site.baseurl}}/images/20160802/sample_message1.png){: .center-image .half }
![answered]({{ site.baseurl}}/images/20160802/sample_message2.png){: .center-image .half }

The joint confirmation & answered notification is somewhat less important than the first message (question) and last message (all answers). So during peak time, this less important message is dropped.
The green line below shows message sent and the orange shows discarded message (dropped). This why on peak the bot response slowly.

So what would be the permanent solutions?  
Currently it's not possible to run multiple bot due to the server limitation. So before that,  I need to optimize the sending message a bit more.
The code is getting quite complex and convoluted adding more into will not go for future maintainability of the code. Thus I need to refactor the core code.

So I'm going to rewrite big part of the code. This will allow it to:

* Handle more traffic
* Sending more efficient message (by doing batching)
* develop more feature (hint: *real survey*)
* the core can be used to develop other game on telegram

But this is going to take some time. Especially when currently I'm doing this my self outside of my daily JOB :).  
I'm just doing this for fun and just for experimentation. I don't get paid, and I'm using my personal time as well. 

**So for now, please be patient and understand the circumstances** since I'll be focusing more effort on this and not adding more question or feature.

Thank you for understanding

PS: if you are go developer. I could need some help. just ping me