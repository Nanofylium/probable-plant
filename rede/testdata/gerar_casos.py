# Gera rede/testdata/casos.json: modelos ONNX pequenos + saída do onnxruntime.
import json, base64, numpy as np, onnx, onnxruntime as ort
from onnx import helper as h, TensorProto as T, numpy_helper as nh
rng = np.random.default_rng(7)
casos = []
def caso(nome, nos, x, inits=(), forma_saida=None):
    g = h.make_graph(nos, nome,
        [h.make_tensor_value_info('x', T.FLOAT, list(x.shape))],
        [h.make_tensor_value_info('y', T.FLOAT, None)],
        [nh.from_array(v, k) for k, v in inits])
    m = h.make_model(g, opset_imports=[h.make_opsetid('', 14)])
    m.ir_version = 7
    b = m.SerializeToString()
    y = ort.InferenceSession(b).run(None, {'x': x})[0].astype(np.float32)
    casos.append({'nome': nome, 'modelo': base64.b64encode(b).decode(), 'forma': list(x.shape), 'x': x.ravel().tolist(),
                  'forma_y': list(y.shape), 'y': y.ravel().tolist()})
f = lambda *s: rng.standard_normal(s).astype(np.float32)
i64 = lambda *v: np.array(v, np.int64)

caso('matmul_add_relu', [h.make_node('MatMul', ['x', 'w'], ['a']), h.make_node('Add', ['a', 'b'], ['c']), h.make_node('Relu', ['c'], ['y'])],
     f(2, 3, 5), [('w', f(5, 4)), ('b', f(4))])
caso('matmul_lote', [h.make_node('Transpose', ['x'], ['t'], perm=[0, 1, 3, 2]), h.make_node('MatMul', ['x', 't'], ['y'])], f(1, 2, 3, 4))
caso('conv_pad_stride', [h.make_node('Conv', ['x', 'w', 'b'], ['y'], kernel_shape=[3, 3], pads=[1, 1, 1, 1], strides=[2, 2])],
     f(1, 2, 7, 6), [('w', f(3, 2, 3, 3)), ('b', f(3))])
caso('conv_1x1', [h.make_node('Conv', ['x', 'w'], ['y'], kernel_shape=[1, 1])], f(1, 4, 5, 5), [('w', f(2, 4, 1, 1))])
caso('conv_transposta', [h.make_node('ConvTranspose', ['x', 'w', 'b'], ['y'], kernel_shape=[2, 2], strides=[2, 2])],
     f(1, 3, 3, 2), [('w', f(3, 2, 2, 2)), ('b', f(2))])
caso('resize_linear_cantos', [h.make_node('Resize', ['x', '', 'e'], ['y'], mode='linear', coordinate_transformation_mode='align_corners')],
     f(1, 2, 3, 4), [('e', np.array([1, 1, 2, 1.5], np.float32))])
caso('resize_cubico_tamanho', [h.make_node('Resize', ['x', '', '', 's'], ['y'], mode='cubic', coordinate_transformation_mode='half_pixel', cubic_coeff_a=-0.75)],
     f(1, 2, 4, 4), [('s', i64(1, 2, 7, 5))])
caso('layernorm_gelu_softmax', [
    h.make_node('ReduceMean', ['x'], ['m'], axes=[-1]), h.make_node('Sub', ['x', 'm'], ['d']),
    h.make_node('Pow', ['d', 'dois'], ['p']), h.make_node('ReduceMean', ['p'], ['v'], axes=[-1]),
    h.make_node('Add', ['v', 'eps'], ['ve']), h.make_node('Sqrt', ['ve'], ['s']), h.make_node('Div', ['d', 's'], ['n']),
    h.make_node('Div', ['n', 'r2'], ['q']), h.make_node('Erf', ['q'], ['e']), h.make_node('Add', ['e', 'um'], ['e1']),
    h.make_node('Mul', ['n', 'e1'], ['g']), h.make_node('Softmax', ['g'], ['y'], axis=-1)],
    f(2, 3, 6), [('dois', np.array(2, np.float32)), ('eps', np.array(1e-6, np.float32)), ('r2', np.array(1.4142135, np.float32)), ('um', np.array(1, np.float32))])
caso('formas', [
    h.make_node('Shape', ['x'], ['sh']), h.make_node('Gather', ['sh', 'i0'], ['n0'], axis=0),
    h.make_node('Slice', ['x', 'st', 'en', 'ax'], ['sl']),
    h.make_node('Unsqueeze', ['n0', 'z'], ['n0u']), h.make_node('Concat', ['n0u', 'menos1'], ['nova'], axis=0),
    h.make_node('Reshape', ['sl', 'nova'], ['r']), h.make_node('Gather', ['r', 'i1'], ['gq'], axis=1),
    h.make_node('Unsqueeze', ['gq', 'z'], ['gu']), h.make_node('Squeeze', ['gu', 'z'], ['y'])],
    f(2, 5, 3), [('i0', np.array(0, np.int64)), ('st', i64(1)), ('en', i64(9223372036854775807)), ('ax', i64(1)),
                 ('z', i64(0)), ('menos1', i64(-1)), ('i1', i64(2, 0))])
caso('expand_where', [
    h.make_node('Shape', ['x'], ['sh']), h.make_node('Equal', ['sh', 'tres'], ['eq']),
    h.make_node('Where', ['eq', 'uns', 'sh'], ['w']), h.make_node('Expand', ['c', 'w'], ['ex']),
    h.make_node('Concat', ['ex', 'x'], ['y'], axis=1)],
    f(1, 3, 4), [('tres', i64(1, 9, 4)), ('uns', i64(1, 1, 1)), ('c', f(1, 1, 4))])
json.dump(casos, open('/home/user/probable-plant/rede/testdata/casos.json', 'w'))
print(len(casos), 'casos')
