select 
  avg(score) as avg_score, 
  count(*) as cnt,
  a.shop_id, name
from goods a
left join shops b 
on a.shop_id = b.shop_id
where 
-- category=50010850 -- 限定连衣裙 不需要，有些档口分类不准确
added_at > '2016-02-01' -- 上新范围
and a.shop_id < 20000 -- 老店
and price >= 60 -- 最低价格
and price <= 150 -- 过滤非正常价格
and status = 1 -- 不要下架商品
group by shop_id -- 按档口分组
having cnt > 10 -- 商品数量
order by avg_score desc -- 平均分最高
limit 30;
