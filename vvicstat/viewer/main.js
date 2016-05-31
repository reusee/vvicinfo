import {div, img, span, button, 
  e, Store, Component} from './base'

class App extends Component {
  render(state) {
    return div({}, [
      ...state.entries.slice(state.page * 100, (state.page + 1) * 100).map((entry) => {
        return e(Entry, entry);
      }),

      // pager
      div({
        style: {
          position: 'fixed',
          bottom: '20px',
          right: '10%',
        },
      }, (() => {
        let ret = [];
        let max_page = state.entries.length / 100;
        for (let page = 0; page <= max_page; page++) {
          ret.push(state.page == page ? span({}, page) : button({
            onclick: () => {
              emit((state) => {
                window.scrollTo(0, 0);
                return {
                  page: page,
                };
              });
            },
          }, page));
        }
        return ret;
      })()),
    ]);
  }
}

class Entry extends Component {
  render(state) {
    return div({}, [
      ...state.images.map(image => {
        return img({
          style: {
            maxWidth: '640px',
            border: '1px solid grey',
            padding: '5px',
          },
          src: image,
        });
      }),
      ...state.goods.map(good => {
        return div({}, [
          span({}, '￥' + good.price),
          e('a', {
            href: 'http://www.vvic.com/item.html?id=' + good.good_id,
            target: '_blank',
          }, 'http://www.vvic.com/item.html?id=' + good.good_id),
        ]);
      }),
    ]);
  }
}

let initState = {
  entries: window.data,
  page: 0,
};
let app = new App(initState);
app.bind(document.getElementById('app'));
let store = new Store(initState);
app.setStore(store);
let emit = store.emit.bind(store);
