import requests
from bs4 import BeautifulSoup
import json

def check_amazon_stock(product_url):
    """
    检测亚马逊商品是否有货。

    Args:
        product_url: 商品页面的URL。

    Returns:
        True 如果商品有货，False 如果商品缺货，None 如果无法确定。
    """
    try:
        headers = {
            'User-Agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36'
        }
        response = requests.get(product_url, headers=headers)
        response.raise_for_status()  # 如果状态码不是200，抛出异常
        soup = BeautifulSoup(response.content, 'html.parser')
        # 保存 soup 的 HTML 到 JSON 文件（Windows 路径修正）
        with open(r'C:\Users\irony\Desktop\amazon_page.json', 'w', encoding='utf-8') as f:
            json.dump({'html': soup.prettify()}, f, ensure_ascii=False, indent=2)

        # 检查是否有“添加到购物车”按钮
        add_to_cart_button = soup.find('input', {'id': 'add-to-cart-button'})
        if add_to_cart_button:
            return True

        # 检查是否有“目前无货”提示（div > span 结构）
        out_of_stock_div = soup.find('div', class_='a-section a-spacing-small a-text-center')
        if out_of_stock_div:
            out_of_stock_span = out_of_stock_div.find('span', class_='a-color-price a-text-bold')
            if out_of_stock_span and "目前无货" in out_of_stock_span.text:
                return False

        # 如果以上两种情况都没找到，则可能无法确定，返回None
        return None

    except requests.exceptions.RequestException as e:
        print(f"请求错误: {e}")
        return None
    except Exception as e:
        print(f"发生错误: {e}")
        return None


# 示例用法
product_url = "https://www.amazon.co.jp/-/zh/dp/B0FB2JZR4T/ref=pd_vtp_h_pd_vtp_h_d_sccl_2/358-7041935-4436444?pd_rd_w=3S3lg&content-id=amzn1.sym.aa2b64e9-bbda-48af-869b-bd011156b91d&pf_rd_p=aa2b64e9-bbda-48af-869b-bd011156b91d&pf_rd_r=KV7GG60GPQRPX0GWYNGP&pd_rd_wg=vyUAK&pd_rd_r=67f9c8be-4594-4c4c-a2ee-7a02d6cd51e8&pd_rd_i=B0FB2JZR4T&psc=1"  # 替换为实际的商品URL
stock_status = check_amazon_stock(product_url)

if stock_status is True:
    print("商品有货")
elif stock_status is False:
    print("商品缺货")
else:
    print("无法确定商品是否有货")